package hotweb

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
	"github.com/progrium/hotweb/pkg/esbuild"
	"github.com/progrium/hotweb/pkg/jsexports"
	"github.com/progrium/hotweb/pkg/makefs"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

const (
	DefaultWatchInterval = time.Millisecond * 100
)

var (
	InternalPath = "/.hotweb"
	ReloadExport = "noHMR"
)

func debug(args ...interface{}) {
	if os.Getenv("HOTWEB_DEBUG") != "" {
		log.Println(append([]interface{}{"hotweb:"}, args...)...)
	}
}

type Handler struct {
	Fs            *makefs.Fs
	ServeRoot     string
	Prefix        string
	IgnoreDirs    []string
	WatchInterval time.Duration

	Upgrader websocket.Upgrader
	Watcher  *watcher.Watcher

	fileserver http.Handler
	clients    sync.Map
	mux        http.Handler
	muxOnce    sync.Once
}

func newWriteWatcher(fs afero.Fs, root string) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetFileSystem(fs)
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)
	return w, w.AddRecursive(root)
}

func New(fs afero.Fs, serveRoot, prefix string) *Handler {
	cache := afero.NewMemMapFs()
	mfs := makefs.New(fs, cache)

	var watcher *watcher.Watcher
	var err error
	watcher, err = newWriteWatcher(fs, serveRoot)
	if err != nil {
		panic(err)
	}

	mfs.Register(".js", ".jsx", func(fs afero.Fs, dst, src string) ([]byte, error) {
		debug("building", dst)
		return esbuild.BuildFile(fs, src)
	})

	httpFs := afero.NewHttpFs(mfs).Dir(serveRoot)
	prefix = path.Join("/", prefix)
	return &Handler{
		Fs:        mfs,
		ServeRoot: serveRoot,
		Prefix:    prefix,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Watcher:       watcher,
		WatchInterval: DefaultWatchInterval,
		fileserver:    http.StripPrefix(prefix, http.FileServer(httpFs)),
	}
}

func (m *Handler) MatchHTTP(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, path.Join(m.Prefix, InternalPath)) {
		return true
	}
	if strings.HasPrefix(r.URL.Path, m.Prefix) {
		fsPath := path.Join(m.ServeRoot, strings.TrimPrefix(r.URL.Path, m.Prefix))
		if ok, _ := afero.Exists(m.Fs, fsPath); ok {
			return true
		}
	}
	return false
}

func (m *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.muxOnce.Do(m.buildMux)
	m.mux.ServeHTTP(w, r)
}

func (m *Handler) buildMux() {
	mux := http.NewServeMux()
	mux.HandleFunc(path.Join(m.Prefix, InternalPath, ClientFilename), m.handleClientModule)
	mux.HandleFunc(path.Join(m.Prefix, InternalPath), m.handleWebSocket)
	if len(m.Prefix) > 1 {
		mux.HandleFunc(m.Prefix+"/", m.handleFileProxy)
	} else {
		mux.HandleFunc(m.Prefix, m.handleFileProxy)
	}
	m.mux = mux
}

func (m *Handler) isValidJS(r *http.Request) bool {
	return !m.isIgnored(r) && isJavaScript(r) && !hiddenFilePrefix(r)
}

func (m *Handler) isIgnored(r *http.Request) bool {
	for _, path := range m.IgnoreDirs {
		if path != "" && strings.HasPrefix(r.URL.Path, path) {
			return true
		}
	}
	return false
}

func (m *Handler) handleFileProxy(w http.ResponseWriter, r *http.Request) {
	if m.isValidJS(r) && r.URL.RawQuery == "" {
		m.handleModuleProxy(w, r)
		return
	}
	m.fileserver.ServeHTTP(w, r)
}

func (m *Handler) handleClientModule(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("client").Parse(ClientSourceTmpl))

	w.Header().Set("content-type", "text/javascript")
	tmpl.Execute(w, map[string]interface{}{
		"Debug":    os.Getenv("HOTWEB_DEBUG") != "",
		"Endpoint": fmt.Sprintf("ws://%s%s", r.Host, path.Dir(r.URL.Path)),
	})
}

func (m *Handler) handleModuleProxy(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("proxy").Parse(ModuleProxyTmpl))

	fsPath := path.Join(m.ServeRoot, strings.TrimPrefix(r.URL.Path, m.Prefix))
	src, err := afero.ReadFile(m.Fs, fsPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		debug(err)
		return
	}

	exports, err := jsexports.Exports(src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		debug(err)
		return
	}

	w.Header().Set("content-type", "text/javascript")
	tmpl.Execute(w, map[string]interface{}{
		"Path":       r.URL.Path,
		"Exports":    exports,
		"Reload":     contains(exports, ReloadExport),
		"ClientPath": path.Join(m.Prefix, InternalPath, ClientFilename),
	})
}

func (m *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close()
	ch := make(chan string)
	m.clients.Store(ch, struct{}{})
	debug("new websocket connection")

	for filepath := range ch {
		err := conn.WriteJSON(map[string]interface{}{
			"path": path.Join(m.Prefix, strings.TrimPrefix(filepath, m.ServeRoot)),
		})
		if err != nil {
			m.clients.Delete(ch)
			if !strings.Contains(err.Error(), "broken pipe") {
				debug(err)
			}
			return
		}
	}
}

func (m *Handler) Watch() error {
	if m.Watcher == nil {
		return fmt.Errorf("hotweb: no watcher to watch filesystem")
	}
	go func() {
		for {
			select {
			case event := <-m.Watcher.Event:
				debug("detected change", event.Path)
				m.clients.Range(func(k, v interface{}) bool {
					k.(chan string) <- event.Path
					return true
				})
			case err := <-m.Watcher.Error:
				debug(err)
			case <-m.Watcher.Closed:
				return
			}
		}
	}()
	return m.Watcher.Start(m.WatchInterval)
}

func isJavaScript(r *http.Request) bool {
	return contains([]string{".mjs", ".js", ".jsx"}, path.Ext(r.URL.Path))
}

func hiddenFilePrefix(r *http.Request) bool {
	return path.Base(r.URL.Path)[0] == '_' || path.Base(r.URL.Path)[0] == '.'
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

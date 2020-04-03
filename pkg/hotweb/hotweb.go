package hotweb

import (
	"fmt"
	"io"
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
	ReloadExport     = "noHMR"
	ClientModuleName = "_hotweb.mjs"
)

func debug(args ...interface{}) {
	if os.Getenv("HOTWEB_DEBUG") != "" {
		log.Println(append([]interface{}{"hotweb:"}, args...)...)
	}
}

type Handler struct {
	Fs            *makefs.Fs
	ServeRoot     string
	IgnoreDirs    []string
	WatchInterval time.Duration

	Upgrader websocket.Upgrader
	Watcher  *watcher.Watcher

	clients sync.Map
}

func newWriteWatcher(fs afero.Fs, root string) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetFileSystem(fs)
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)
	return w, w.AddRecursive(root)
}

func New(fs afero.Fs, serveRoot string) *Handler {
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

	return &Handler{
		Fs:        mfs,
		ServeRoot: serveRoot,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Watcher:       watcher,
		WatchInterval: DefaultWatchInterval,
	}
}

func (m *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: some way to make sure this is hotweb websocket
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := m.Upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		m.handleWebSocket(conn)
		return
	}

	if path.Base(r.URL.Path) == ClientModuleName {
		m.handleClientModule(w, r)
		return
	}

	if m.isValidJS(r) && r.URL.RawQuery == "" {
		m.handleModuleProxy(w, r)
		return
	}

	httpFs := afero.NewHttpFs(m.Fs)
	fileserver := http.FileServer(httpFs.Dir(m.ServeRoot))
	fileserver.ServeHTTP(w, r)
}

func (m *Handler) isValidJS(r *http.Request) bool {
	return !m.isIgnored(r) && isJavaScript(r) && !underscoreFilePrefix(r)
}

func (m *Handler) isIgnored(r *http.Request) bool {
	for _, path := range m.IgnoreDirs {
		if path != "" && strings.HasPrefix(r.URL.Path, path) {
			return true
		}
	}
	return false
}

func (m *Handler) handleClientModule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/javascript")
	debug := "false"
	if os.Getenv("HOTWEB_DEBUG") != "" {
		debug = "true"
	}
	io.WriteString(w, fmt.Sprintf(ClientModule, debug))
}

func (m *Handler) handleModuleProxy(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("proxy").Parse(ModuleProxyTmpl))

	filepath := path.Join(m.ServeRoot, r.URL.Path)
	src, err := afero.ReadFile(m.Fs, filepath)
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
		"ClientPath": ClientModuleName,
	})
}

func (m *Handler) handleWebSocket(conn *websocket.Conn) {
	defer conn.Close()
	ch := make(chan string)
	m.clients.Store(ch, struct{}{})
	debug("new websocket connection")

	for path := range ch {
		err := conn.WriteJSON(map[string]interface{}{
			"path": strings.TrimPrefix(path, m.ServeRoot),
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

func underscoreFilePrefix(r *http.Request) bool {
	return path.Base(r.URL.Path)[0] == '_'
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

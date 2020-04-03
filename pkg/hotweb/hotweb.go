package hotweb

import (
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
	"github.com/progrium/hotweb/pkg/esbuild"
	"github.com/progrium/hotweb/pkg/jsexports"
	"github.com/progrium/hotweb/pkg/makefs"
	"github.com/radovskyb/watcher"
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
	log.Println(args...)
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

func newWriteWatcher(filepath string) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)
	return w, w.AddRecursive(filepath)
}

func New(fs afero.Fs, serveRoot string, watch bool) *Handler {
	cache := afero.NewMemMapFs()
	mfs := makefs.New(fs, cache)

	var watcher *watcher.Watcher
	var err error
	if watch {
		watcher, err = newWriteWatcher(serveRoot)
		if err != nil {
			panic(err)
		}
	}

	mfs.Register(".js", ".jsx", func(fs afero.Fs, dst, src string) error {
		b, err := esbuild.BuildFile(fs, src)
		if err != nil {
			return err
		}
		return afero.WriteFile(fs, dst, b, 0644)
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

	// if m.isValidJS(r) && m.jsxExists(r) && r.URL.RawQuery != "" {
	// 	m.handleBuildJsx(w, r)
	// 	return
	// }

	if m.isValidJS(r) && r.URL.RawQuery == "" {
		m.handleModuleProxy(w, r)
		return
	}

	httpFs := afero.NewHttpFs(m.Fs)
	fileserver := http.FileServer(httpFs.Dir(m.ServeRoot))
	fileserver.ServeHTTP(w, r)
}

// func (m *Handler) jsxExists(r *http.Request) bool {
// 	if _, err := os.Stat(m.jsxPath(r)); os.IsNotExist(err) {
// 		return false
// 	}
// 	return true
// }

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
	io.WriteString(w, ClientModule)
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
	//debug("new hotweb connection")

	for path := range ch {
		err := conn.WriteJSON(map[string]interface{}{
			"path": strings.TrimPrefix(path, m.ServeRoot),
		})
		if err != nil {
			m.clients.Delete(ch)
			if !strings.Contains(err.Error(), "broken pipe") {
				debug("hotweb error:", err)
			}
			return
		}
	}
}

func (m *Handler) Watch() error {
	if m.Watcher == nil {
		debug("hotweb: unable to watch filesystem")
		return nil
	}
	go func() {
		for {
			select {
			case event := <-m.Watcher.Event:
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

// func (m *Handler) jsxPath(r *http.Request) string {
// 	return filepath.Clean(filepath.Join(m.Fileroot, r.URL.Path)) + "x" // TODO: dont cheat
// }

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

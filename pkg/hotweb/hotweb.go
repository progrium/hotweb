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
	"github.com/progrium/hotweb/pkg/jsexports"
	"github.com/radovskyb/watcher"
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

type Middleware struct {
	Fileroot      string
	WatchInterval time.Duration

	Upgrader websocket.Upgrader
	Watcher  *watcher.Watcher

	clients sync.Map
	next    http.Handler
}

func newWriteWatcher(filepath string) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)
	return w, w.AddRecursive(filepath)
}

func New(fileroot string, next http.Handler) *Middleware {
	watcher, err := newWriteWatcher(fileroot)
	if err != nil {
		panic(err)
	}
	if next == nil {
		next = http.FileServer(http.Dir(fileroot))
	}
	return &Middleware{
		Fileroot: fileroot,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Watcher:       watcher,
		WatchInterval: DefaultWatchInterval,
		next:          next,
	}
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if isJavaScript(r) && !underscoreFilePrefix(r) && r.URL.RawQuery == "" {
		m.handleModuleProxy(w, r)
		return
	}

	m.next.ServeHTTP(w, r)
}

func (m *Middleware) handleClientModule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/javascript")
	io.WriteString(w, ClientModule)
}

func (m *Middleware) handleModuleProxy(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("proxy").Parse(ModuleProxyTmpl))

	exports, err := jsexports.Exports(path.Join(m.Fileroot, r.URL.Path))
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

func (m *Middleware) handleWebSocket(conn *websocket.Conn) {
	defer conn.Close()
	ch := make(chan string)
	m.clients.Store(ch, struct{}{})
	debug("new hotweb connection")

	for path := range ch {
		err := conn.WriteJSON(map[string]interface{}{
			"path": strings.TrimPrefix(path, m.Fileroot),
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

func (m *Middleware) Watch() error {
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

func isJavaScript(r *http.Request) bool {
	return contains([]string{".mjs", ".js"}, path.Ext(r.URL.Path))
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

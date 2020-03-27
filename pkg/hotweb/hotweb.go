package hotweb

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
	"github.com/progrium/hotweb/pkg/esbuild"
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
	IgnoreDirs    []string
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
	fileroot, err = filepath.Abs(fileroot)
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

	if m.isValidJS(r) && m.jsxExists(r) && r.URL.RawQuery != "" {
		m.handleBuildJsx(w, r)
		return
	}

	if m.isValidJS(r) && r.URL.RawQuery == "" {
		m.handleModuleProxy(w, r)
		return
	}

	m.next.ServeHTTP(w, r)
}

func (m *Middleware) jsxExists(r *http.Request) bool {
	if _, err := os.Stat(m.jsxPath(r)); os.IsNotExist(err) {
		return false
	}
	return true
}

func (m *Middleware) isValidJS(r *http.Request) bool {
	return !m.isIgnored(r) && isJavaScript(r) && !underscoreFilePrefix(r)
}

func (m *Middleware) isIgnored(r *http.Request) bool {
	for _, path := range m.IgnoreDirs {
		if path != "" && strings.HasPrefix(r.URL.Path, path) {
			return true
		}
	}
	return false
}

func (m *Middleware) handleBuildJsx(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/javascript")
	b, err := esbuild.BuildFile(m.jsxPath(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *Middleware) handleClientModule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/javascript")
	io.WriteString(w, ClientModule)
}

func (m *Middleware) handleModuleProxy(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("proxy").Parse(ModuleProxyTmpl))

	filepath := path.Join(m.Fileroot, r.URL.Path)
	if m.jsxExists(r) {
		filepath += "x"
	}

	exports, err := jsexports.Exports(filepath)
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

func (m *Middleware) jsxPath(r *http.Request) string {
	return filepath.Clean(filepath.Join(m.Fileroot, r.URL.Path)) + "x" // TODO: dont cheat
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

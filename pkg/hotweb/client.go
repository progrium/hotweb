package hotweb

var ClientModule = `
let listeners = {};
let refreshers = [];
let ws = undefined;
 
(function connect() {
    ws = new WebSocket(import.meta.url.replace("http", "ws"));
    ws.onopen = () => console.debug("hotweb websocket open");
    ws.onclose = () => console.debug("hotweb websocket closed");
    ws.onerror = (err) => console.debug("hotweb websocket error: ", err);
    ws.onmessage = async (event) => {
        let msg = JSON.parse(event.data);
        let paths = Object.keys(listeners);
        paths.sort((a, b) => b.length - a.length);
        for (const idx in paths) {
            let path = paths[idx];
            if (msg.path.startsWith(path)) {
                for (const i in listeners[path]) {
                    await listeners[path][i]((new Date()).getTime(), msg.path);
                }
            }
        }
        // wtf why aren't refreshers consistently 
        // run after listeners are called.
        // setTimeout workaround seems ok for now
        setTimeout(() => refreshers.forEach((cb) => cb()), 20);
    }; 
})();  

export function accept(path, cb) {
    if (listeners[path] === undefined) {
        listeners[path] = [];
    }
    listeners[path].push(cb);
}

export function refresh(cb) {
    refreshers.push(cb);
    cb();
}

export function watchHTML() {
    let withIndex = "";
    if (location.pathname[location.pathname.length-1] == "/") {
        withIndex = location.pathname + "index.html";
    } else {
        withIndex = location.pathname + "/index.html";
    }
    accept(location.pathname, (ts, path) => {
        if (path == location.pathname || path == withIndex) {
            location.reload();
        }
    });
}

export function watchCSS() {
    accept("", (ts, path) => {
        if (path.endsWith(".css")) {
            let link = document.createElement('link');
            link.setAttribute('rel', 'stylesheet');
            link.setAttribute('type', 'text/css');
            link.setAttribute('href', path+'?'+(new Date()).getTime());
            document.getElementsByTagName('head')[0].appendChild(link);
            let styles = document.getElementsByTagName("link");
            for (let i=0; i<styles.length; i++) {
                if (i < styles.length-1 && styles[i].getAttribute("href").startsWith(path)) {
                    styles[i].remove();
                }
            }
        }
    });
}

`

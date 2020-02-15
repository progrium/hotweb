# hotweb
Live reloading and ES6 hot module replacement for plain old JavaScript

Although a number of tools exist for live development, this tool was 
created for per module reloading, specifically ES6 modules. 
It also reloads CSS and full pages on HTML changes if desired.

When used with plain old JavaScript component frameworks like 
[Mithril](https://mithril.js.org/), you can finally have modern, component-based frontend
development **without a compile step, without Node.js or any node_modules,
and without Webpack.**

## Getting hotweb
```
$ go install github.com/progrium/hotweb/cmd/hotweb
```

## Quickstart with example
The `_example` directory contains a small Mithril+Bulma application. I took a free
layout and broke it into Mithril components. You can use this to test the reloading
capabilities. Just run `hotweb` in the `_example` directory and start changing HTML, CSS,
or JavaScript. You'll notice changing the JavaScript updates the browser without reloading
the page.

## Using hotweb

### Setting up the hotweb JS client
Add this line to your main Javascript module:
```javascript
import * as hotweb from '/_hotweb.mjs';
```
Now any JavaScript loaded will be reloaded when their files are changed.
There is a callback for when a reload occurs so you can trigger whatever needs
to be re-evaluated with the reloaded modules. For example, with Mithril this
is where you would call `m.redraw()`:
```javascript
hotweb.refresh(() => m.redraw());
```
To enable full page reloads on HTML changes:
```javascript
hotweb.watchHTML();
```
To enable CSS hot reloads:
```javascript
hotweb.watchCSS();
```

### Running the hotweb server
Run hotweb in the web root you'd like to serve:
```
$ hotweb
```
It will open a browser to the index and files in the directory will be watched.
You can also specify a different path to serve or a different port. See `hotweb -h`.

### Using the hotweb package
The hotweb server is just a little command line tool wrapping the hotweb package,
which you can use directly in Go to customize or integrate hotweb with your tooling.

[GoDocs](https://godoc.org/github.com/progrium/hotweb/pkg/hotweb)

## Notes

### Stateful JS modules
You may experience weird bugs if you try to hot replace stateful modules. You can 
mark a module to reload the whole page instead of trying to hot replace by exporting
a field named `noHMR`. The type and value are ignored. Example:
```javascript
export const noHTML = true;
```

### Root components
The way we implement HMR with on-the-fly generated proxy modules means in order to pick
up the new module exports, you need to access them through the imported names. When we
redraw in Mithril it will call render on the top level component, and as it references
subcomponents they will be updated references. However, because the root component is not
re-evaluated, it will not update with changes unless you wrap it in a callback so it
gets evaluated again.

For example, say you mount your top level component like this:
```javascript
import * as app from '/lib/app.mjs';

m.mount(document.body, app.Page));
```
To get `Page` to update when `app.mjs` is modified we have to wrap it so accessing
`Page` happens with every render. Something like this:
```javascript
import * as app from '/lib/app.mjs';

m.mount(document.body, wrap(() => app.Page));

function wrap(cb) {
    return {view: () => m(cb())};
}
```

## License
MIT
package hotweb

var ModuleProxyTmpl = `import * as hotweb from '{{.ClientPath}}';
import * as mod from '{{.Path}}?0';

{{range .Exports}}let {{.}}Proxy = mod.{{.}};
{{end}}

hotweb.accept('{{.Path}}', async (ts) => {
{{ if .Reload }}	location.reload();
{{ else }}	let newMod = await import("{{.Path}}?"+ts);
{{range .Exports}}	{{.}}Proxy = newMod.{{.}};
{{end}}
{{- end -}}
});

export {
{{range .Exports}}	{{.}}Proxy as {{.}},
{{end}}
};
`

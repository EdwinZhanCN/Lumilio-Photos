// Command problemcatalog publishes the stable documentation behind Lumilio's
// RFC 9457 type URI namespace.
package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"server/internal/api/problem"
)

type pageData struct {
	Descriptor problem.Descriptor
	Path       string
}

var pageTemplate = template.Must(template.New("problem").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{.Descriptor.Title}} · Lumilio API Problem</title>
  <style>body{max-width:52rem;margin:4rem auto;padding:0 1.5rem;font:16px/1.6 system-ui,sans-serif;color:#202124}code{background:#f3f4f6;padding:.15rem .35rem;border-radius:.25rem}dt{font-weight:700;margin-top:1rem}dd{margin-left:0}table{border-collapse:collapse;width:100%}th,td{text-align:left;border-bottom:1px solid #ddd;padding:.6rem}</style>
</head>
<body>
  <p><a href="/problems/">Lumilio API Problems</a></p>
  <h1>{{.Descriptor.Title}}</h1>
  <dl>
    <dt>Type</dt><dd><code>{{.Descriptor.Type}}</code></dd>
    <dt>Normal HTTP status</dt><dd>{{.Descriptor.Status}}</dd>
    <dt>Definition</dt><dd>{{.Descriptor.Definition}}</dd>
    <dt>Recovery</dt><dd>{{.Descriptor.Recovery}}</dd>
  </dl>
  <h2>Extension members</h2>
  {{if .Descriptor.Extensions}}<table><thead><tr><th>Name</th><th>Schema</th><th>Meaning</th></tr></thead><tbody>{{range .Descriptor.Extensions}}<tr><td><code>{{.Name}}</code></td><td>{{.Schema}}</td><td>{{.Description}}</td></tr>{{end}}</tbody></table>{{else}}<p>This Problem type defines no extension members.</p>{{end}}
  <p>Runtime clients identify this condition by <code>type</code>; they do not dereference this page automatically.</p>
</body>
</html>
`))

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Lumilio API Problems</title><style>body{max-width:52rem;margin:4rem auto;padding:0 1.5rem;font:16px/1.6 system-ui,sans-serif;color:#202124}code{background:#f3f4f6;padding:.15rem .35rem;border-radius:.25rem}li{margin:.7rem 0}</style></head>
<body><h1>Lumilio API Problems</h1><p>Stable RFC 9457 Problem type documentation for the Lumilio API.</p><ul>{{range .}}<li><a href="/problems/{{.Path}}/"><code>{{.Descriptor.Type}}</code></a> — {{.Descriptor.Definition}}</li>{{end}}</ul></body></html>
`))

func main() {
	if len(os.Args) != 2 {
		panic("usage: problemcatalog OUTPUT_DIRECTORY")
	}
	output := filepath.Clean(os.Args[1])
	if filepath.Base(output) != "problems" {
		panic("output directory must end in problems")
	}
	if err := os.RemoveAll(output); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		panic(err)
	}

	descriptors := problem.Registered()
	pages := make([]pageData, 0, len(descriptors))
	for _, descriptor := range descriptors {
		path := strings.TrimPrefix(descriptor.Type, problem.Namespace)
		page := pageData{Descriptor: descriptor, Path: path}
		pages = append(pages, page)
		directory := filepath.Join(output, filepath.FromSlash(path))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			panic(err)
		}
		write(filepath.Join(directory, "index.html"), pageTemplate, page)
	}
	write(filepath.Join(output, "index.html"), indexTemplate, pages)
	fmt.Printf("Published %d Problem type pages in %s.\n", len(pages), output)
}

func write(path string, tmpl *template.Template, value any) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(file, value); err != nil {
		_ = file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}

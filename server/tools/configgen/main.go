// Command configgen regenerates the manifest JSON Schema and the per-scenario
// example manifests from the profile table in server/config.
//
// Both outputs are derived artifacts. The manifest struct, its Go doc comments,
// and the profile table are the source of truth; a golden test fails if the
// checked-in files drift from what this command produces.
//
// Usage:
//
//	go run ./tools/configgen            # write the files
//	go run ./tools/configgen -check     # fail if they are stale
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"server/config"
)

func main() {
	check := flag.Bool("check", false, "verify the checked-in files are current instead of writing them")
	root := flag.String("root", "config", "path to the server/config package")
	flag.Parse()

	if err := run(*root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "configgen:", err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	artifacts, err := build(root)
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(artifacts))
	for path := range artifacts {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var stale []string
	for _, path := range paths {
		want := artifacts[path]
		if check {
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				stale = append(stale, path)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}

	if len(stale) != 0 {
		return fmt.Errorf(
			"%d generated file(s) are stale; run `make config-examples`:\n  %s",
			len(stale), joinLines(stale),
		)
	}
	return nil
}

// build renders every artifact in memory first. The schema has to be written
// before examples can be rendered from it in a fresh checkout, but within one
// process the embedded schema is whatever was compiled in — so a schema change
// needs two runs to fully settle, and -check is what catches that.
func build(root string) (map[string][]byte, error) {
	schema, err := config.GenerateJSONSchema()
	if err != nil {
		return nil, err
	}
	artifacts := map[string][]byte{
		filepath.Join(root, config.SchemaFile): schema,
	}

	examples, err := config.RenderExamples()
	if err != nil {
		return nil, err
	}
	for name, data := range examples {
		artifacts[filepath.Join(root, "examples", filepath.FromSlash(name))] = data
	}
	return artifacts, nil
}

func joinLines(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += "\n  "
		}
		out += value
	}
	return out
}

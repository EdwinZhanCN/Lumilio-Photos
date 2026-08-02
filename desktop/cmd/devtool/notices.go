package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type noticeFile struct{ Name, Text string }
type noticeEntry struct {
	Title, Source, Declared string
	Files                   []noticeFile
}
type goPkg struct{ Module *goModule }
type goModule struct {
	Path, Version, Dir string
	Main               bool
	Replace            *goModule
}
type npmPackage struct {
	Name, Version string
	Private       bool
	License       any
	Homepage      string
	Repository    any
}

func generateThirdPartyNotices(ctx context.Context, root string) error {
	entries := map[string]noticeEntry{}
	add := func(k string, e noticeEntry) { entries[k] = e }
	for _, moduleDir := range []string{filepath.Join(root, "desktop"), filepath.Join(root, "server")} {
		out, err := outputCmd(ctx, moduleDir, nil, "go", "list", "-deps", "-json", "./...")
		if err != nil {
			return err
		}
		dec := json.NewDecoder(strings.NewReader(string(out)))
		mods := map[string]*goModule{}
		for {
			var p goPkg
			if err := dec.Decode(&p); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if p.Module != nil {
				mods[p.Module.Path+"@"+p.Module.Version] = p.Module
			}
		}
		for _, m := range mods {
			if m.Main || m.Path == "server" {
				continue
			}
			actual := m
			if m.Replace != nil {
				actual = m.Replace
			}
			if actual.Dir == "" {
				continue
			}
			v := m.Version
			if v == "" {
				v = actual.Version
			}
			if v == "" {
				v = "local"
			}
			files, err := licenseFiles(actual.Dir)
			if err != nil {
				return err
			}
			add("go:"+m.Path+"@"+v, noticeEntry{Title: strings.TrimSpace(m.Path + " " + v), Source: m.Path, Files: files})
		}
	}
	add("vendored:sqlite-vec1@0.7", noticeEntry{Title: "SQLite Vec1 0.7", Source: "https://sqlite.org/vec1", Declared: "Public Domain", Files: []noticeFile{{Name: "PUBLIC-DOMAIN", Text: "The author disclaims copyright to this source code. The source is dedicated to the public domain."}}})
	nodeModules := filepath.Join(root, "web", "node_modules")
	if !isDir(nodeModules) {
		return fmt.Errorf("web/node_modules is missing; run `cd web && vp install`")
	}
	if err := visitNodeModules(nodeModules, func(dir string, p npmPackage) error {
		if p.Name == "" || p.Private {
			return nil
		}
		source := npmSource(p)
		declared, _ := p.License.(string)
		files, err := licenseFiles(dir)
		if err != nil {
			return err
		}
		add("npm:"+p.Name+"@"+p.Version, noticeEntry{Title: strings.TrimSpace(p.Name + " " + p.Version), Source: source, Declared: declared, Files: files})
		return nil
	}); err != nil {
		return err
	}
	list := make([]noticeEntry, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Title < list[j].Title })
	var b strings.Builder
	b.WriteString("THIRD-PARTY SOFTWARE NOTICES\n============================\n\nLumilio Photos incorporates the following third-party software. This file is generated; do not edit it manually.\n\n")
	for _, e := range list {
		b.WriteString("--------------------------------------------------------------------------------\n")
		b.WriteString(e.Title + "\n")
		if e.Declared != "" {
			b.WriteString("Declared license: " + e.Declared + "\n")
		}
		b.WriteString("Source: " + e.Source + "\n\n")
		if len(e.Files) == 0 {
			b.WriteString("No license text was present in the distributed package metadata; consult the source link above.\n\n")
		}
		for _, f := range e.Files {
			b.WriteString("[" + filepath.Base(f.Name) + "]\n" + f.Text + "\n\n")
		}
	}
	output := filepath.Join(root, "desktop", "licenses", "THIRD_PARTY_NOTICES.txt")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, []byte(strings.TrimRight(b.String(), "\n")+"\n"), 0644); err != nil {
		return err
	}
	fmt.Printf("Wrote %s with %d dependency entries.\n", output, len(entries))
	return nil
}
func licenseFiles(dir string) ([]noticeFile, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []noticeFile
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		base := strings.SplitN(n, ".", 2)[0]
		if base != "license" && base != "licence" && base != "copying" && base != "notice" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		s := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
		sc := bufio.NewScanner(strings.NewReader(s))
		var lines []string
		for sc.Scan() {
			lines = append(lines, strings.TrimRight(sc.Text(), " \t"))
		}
		out = append(out, noticeFile{Name: e.Name(), Text: strings.TrimSpace(strings.Join(lines, "\n"))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func visitNodeModules(dir string, fn func(string, npmPackage) error) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if e.IsDir() && strings.HasPrefix(e.Name(), "@") {
			if err := visitNodeModules(p, fn); err != nil {
				return err
			}
			continue
		}
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			return err
		}
		m := filepath.Join(real, "package.json")
		data, err := os.ReadFile(m)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		var pkg npmPackage
		if err := json.Unmarshal(data, &pkg); err != nil {
			return err
		}
		if err := fn(real, pkg); err != nil {
			return err
		}
	}
	return nil
}
func npmSource(p npmPackage) string {
	switch r := p.Repository.(type) {
	case string:
		if r != "" {
			return r
		}
	case map[string]any:
		if u, ok := r["url"].(string); ok && u != "" {
			return u
		}
	}
	if p.Homepage != "" {
		return p.Homepage
	}
	return "https://www.npmjs.com/package/" + p.Name
}

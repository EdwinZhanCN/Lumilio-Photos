package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed site
var siteAssets embed.FS

type siteGroup struct {
	Group
	Layer int `json:"layer"`
}

type siteView struct {
	*View
	Rows []sequenceRow `json:"rows,omitempty"`
}

type siteSymbol struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Module string `json:"module"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
}

type siteData struct {
	Repo struct {
		Root   string `json:"root"`
		GitHub string `json:"github"`
		Branch string `json:"branch"`
	} `json:"repo"`
	Groups   []siteGroup                `json:"groups"`
	Modules  []*Module                  `json:"modules"`
	Views    []siteView                 `json:"views"`
	Anchors  map[string]*ResolvedAnchor `json:"anchors"`
	Usage    map[string][]string        `json:"usage"`
	Problems []Problem                  `json:"problems"`
	Stale    []Problem                  `json:"stale"`
	Symbols  []siteSymbol               `json:"symbols"`
}

func (a *Atlas) writeSite(target string) error {
	data := siteData{
		Modules:  a.sortedModules(),
		Anchors:  a.Anchors,
		Usage:    a.Usage,
		Problems: a.Problems,
		Stale:    a.Stale,
	}
	data.Repo.Root = a.Root
	data.Repo.GitHub = a.Config.GitHub
	data.Repo.Branch = a.Config.Branch
	layers := groupLayers(a.Config.Groups)
	for _, group := range a.Config.Groups {
		data.Groups = append(data.Groups, siteGroup{Group: group, Layer: layers[group.ID]})
	}
	for _, view := range a.Views {
		item := siteView{View: view}
		if view.Kind == "sequence" {
			item.Rows = sequenceRows(view)
		}
		data.Views = append(data.Views, item)
	}
	for _, symbol := range a.Symbols {
		if !symbol.Exported || symbol.Kind == "field" {
			continue
		}
		data.Symbols = append(data.Symbols, siteSymbol{
			Key: symbol.Key, Name: symbol.Name, Module: symbol.Module,
			File: symbol.File, Line: symbol.Line, Kind: symbol.Kind,
		})
	}
	sort.Slice(data.Symbols, func(i, j int) bool { return data.Symbols[i].Key < data.Symbols[j].Key })
	if data.Problems == nil {
		data.Problems = []Problem{}
	}
	if data.Stale == nil {
		data.Stale = []Problem{}
	}

	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	assets, _ := fs.Sub(siteAssets, "site")
	if err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, name), content, 0o644)
	}); err != nil {
		return err
	}
	mermaid, err := os.ReadFile(filepath.Join(a.Root, "web/node_modules/mermaid/dist/mermaid.min.js"))
	if err != nil {
		return fmt.Errorf("read mermaid (run `task web:install`): %w", err)
	}
	if err := os.WriteFile(filepath.Join(target, "mermaid.min.js"), mermaid, 0o644); err != nil {
		return err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	script := "window.ATLAS = " + strings.ReplaceAll(string(encoded), "</", "<\\/") + ";\n"
	return os.WriteFile(filepath.Join(target, "atlas-data.js"), []byte(script), 0o644)
}

func serveSite(dir, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("atlas: browse http://%s/\n", listener.Addr())
	handler := http.FileServer(http.Dir(dir))
	return http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		handler.ServeHTTP(w, r)
	}))
}

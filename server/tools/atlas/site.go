package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

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
}

// writeData writes the site model for the VitePress Atlas: atlas.json holds
// everything a page needs to render, and atlas-symbols.json the search index,
// which the site loads only when search opens.
func (a *Atlas) writeData(target string) error {
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
	if data.Problems == nil {
		data.Problems = []Problem{}
	}
	if data.Stale == nil {
		data.Stale = []Problem{}
	}
	var symbols []siteSymbol
	for _, symbol := range a.Symbols {
		if !symbol.Exported || symbol.Kind == "field" {
			continue
		}
		symbols = append(symbols, siteSymbol{
			Key: symbol.Key, Name: symbol.Name, Module: symbol.Module,
			File: symbol.File, Line: symbol.Line, Kind: symbol.Kind,
		})
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].Key < symbols[j].Key })
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for name, value := range map[string]any{"atlas.json": data, "atlas-symbols.json": symbols} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, name), encoded, 0o644); err != nil {
			return err
		}
	}
	return nil
}

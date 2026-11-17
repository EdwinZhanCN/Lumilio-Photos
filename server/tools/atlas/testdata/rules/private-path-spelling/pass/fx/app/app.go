package app

import "fx/layout"

// Info has a struct tag that mentions the tree; tags are not checked.
type Info struct {
	Path string `example:".lumilio/logs"`
}

// Logs asks the layout package.
func Logs() string { return layout.Logs() }

// Marker is a public marker, which is not the private tree.
func Marker() string { return ".lumiliorepo" }

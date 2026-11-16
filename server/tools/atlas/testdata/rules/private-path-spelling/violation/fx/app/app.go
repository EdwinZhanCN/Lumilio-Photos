package app

import "path"

// trashDir spells a private path at package level.
const trashDir = ".lumilio/trash"

// Probe is a temporary name.
var Probe = ".lumilio-probe-"

// Info has a struct tag that mentions the tree; tags are not checked.
type Info struct {
	Path string `example:".lumilio/logs"`
}

// Join builds a private path by hand.
func Join(root string) string { return path.Join(root, ".lumilio", "cache") + trashDir }

// Nested names a deeper path.
func Nested() string { return "repo/.lumilio/work" }

// Marker is a public marker, which is not the private tree.
func Marker() string { return ".lumiliorepo" }

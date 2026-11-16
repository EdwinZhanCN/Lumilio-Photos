package storage

import "server/internal/storage/layout"

var (
	ErrRepositoryPathInvalid   = layout.ErrPathInvalid
	ErrRepositoryPathNamespace = layout.ErrPathNamespace
)

// RepositoryPath is a validated, canonical, slash-separated path relative to
// one repository root. The layout package owns the type: media paths come
// from [ParseUserMediaPath], private paths from the layout constructors.
//
// Repository layout Phase 2 moves every caller onto [layout.Path] and the
// layout constructors and deletes this alias and the two parsers below.
type RepositoryPath = layout.Path

// ParseUserMediaPath validates a path in the user-controlled repository tree.
// inbox is an ordinary user directory; the names [layout.IsReserved] reports
// are not media.
func ParseUserMediaPath(value string) (RepositoryPath, error) {
	return layout.ParseMedia(value)
}

// ParsePrivateRepositoryPath validates a path owned by Lumilio under
// .lumilio/. Ordinary media consumers must not receive this capability.
func ParsePrivateRepositoryPath(value string) (RepositoryPath, error) {
	return layout.Parse(value)
}

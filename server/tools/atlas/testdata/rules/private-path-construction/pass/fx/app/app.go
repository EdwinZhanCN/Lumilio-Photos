package app

import (
	"fx/layout"
	"fx/storage"
)

// Logs uses the typed constructor.
func Logs() storage.RepositoryPath { return layout.Logs() }

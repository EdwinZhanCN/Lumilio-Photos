package layout

import "fx/storage"

// Logs is a typed constructor.
func Logs() storage.RepositoryPath { return storage.ParsePrivateRepositoryPath("logs") }

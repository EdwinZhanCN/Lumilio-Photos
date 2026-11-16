package app

import "fx/storage"

// Trash parses a private path by hand.
func Trash() storage.RepositoryPath { return storage.ParsePrivateRepositoryPath("trash") }

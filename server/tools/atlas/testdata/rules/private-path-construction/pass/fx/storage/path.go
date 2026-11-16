package storage

// RepositoryPath is a repository-relative path.
type RepositoryPath struct{ value string }

// ParsePrivateRepositoryPath parses a private path.
func ParsePrivateRepositoryPath(value string) RepositoryPath { return RepositoryPath{value: value} }

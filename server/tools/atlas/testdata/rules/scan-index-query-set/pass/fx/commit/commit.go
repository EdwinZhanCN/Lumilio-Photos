package commit

import "fx/repo"

// Queries is built by the coordinator.
func Queries() *repo.Queries { return repo.New() }

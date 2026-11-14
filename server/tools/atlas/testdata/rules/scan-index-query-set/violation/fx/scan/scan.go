package scan

import "fx/repo"

// Build makes its own query set.
func Build() *repo.Queries { return repo.New() }

package dbtypes

type RepoStatus string

const (
	RepoStatusActive   RepoStatus = "active"
	RepoStatusScanning RepoStatus = "scanning"
	RepoStatusError    RepoStatus = "error"
	RepoStatusOffline  RepoStatus = "offline"
)

type RepoRole string

const (
	RepoRolePrimary RepoRole = "primary"
	RepoRoleRegular RepoRole = "regular"
)

type RepositoryRootKind string

const (
	RepositoryRootKindDefault  RepositoryRootKind = "default"
	RepositoryRootKindExternal RepositoryRootKind = "external"
)

type RepositoryRootStatus string

const (
	RepositoryRootStatusActive  RepositoryRootStatus = "active"
	RepositoryRootStatusOffline RepositoryRootStatus = "offline"
	RepositoryRootStatusError   RepositoryRootStatus = "error"
)

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

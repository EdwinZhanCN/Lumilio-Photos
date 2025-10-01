package dbtypes

type RepoStatus string

const (
	RepoStatusActive   RepoStatus = "active"
	RepoStatusScanning RepoStatus = "scanning"
	RepoStatusError    RepoStatus = "error"
	RepoStatusOffline  RepoStatus = "offline"
)

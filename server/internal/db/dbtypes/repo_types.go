package dbtypes

type RepositoryReachability string

const (
	RepositoryReachabilityActive           RepositoryReachability = "active"
	RepositoryReachabilityOffline          RepositoryReachability = "offline"
	RepositoryReachabilityIdentityError    RepositoryReachability = "identity_error"
	RepositoryReachabilityRecoveryRequired RepositoryReachability = "recovery_required"
	RepositoryReachabilityMaintenance      RepositoryReachability = "maintenance"
)

type RepositoryActivity string

const (
	RepositoryActivityIdle       RepositoryActivity = "idle"
	RepositoryActivityScanning   RepositoryActivity = "scanning"
	RepositoryActivityImporting  RepositoryActivity = "importing"
	RepositoryActivityProcessing RepositoryActivity = "processing"
	RepositoryActivityPaused     RepositoryActivity = "paused"
)

type RepoRole string

const (
	RepoRolePrimary RepoRole = "primary"
	RepoRoleRegular RepoRole = "regular"
)

type StorageLocationKind string

const (
	StorageLocationKindDefault  StorageLocationKind = "default"
	StorageLocationKindExternal StorageLocationKind = "external"
)

type StorageLocationStatus string

const (
	StorageLocationStatusActive      StorageLocationStatus = "active"
	StorageLocationStatusOffline     StorageLocationStatus = "offline"
	StorageLocationStatusError       StorageLocationStatus = "error"
	StorageLocationStatusMaintenance StorageLocationStatus = "maintenance"
)

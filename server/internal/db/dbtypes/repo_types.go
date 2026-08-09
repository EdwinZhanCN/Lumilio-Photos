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

type RepositoryRootKind string

const (
	RepositoryRootKindDefault  RepositoryRootKind = "default"
	RepositoryRootKindExternal RepositoryRootKind = "external"
)

type RepositoryRootStatus string

const (
	RepositoryRootStatusActive      RepositoryRootStatus = "active"
	RepositoryRootStatusOffline     RepositoryRootStatus = "offline"
	RepositoryRootStatusError       RepositoryRootStatus = "error"
	RepositoryRootStatusMaintenance RepositoryRootStatus = "maintenance"
)

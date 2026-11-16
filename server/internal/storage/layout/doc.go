// Package layout is the one declaration of a Repository's tree: it builds,
// classifies, initializes, and validates every path under .lumilio/, and it
// owns the repository-relative [Path] type that the Repository filesystem
// accepts.
//
// # Classes
//
// The first segment under .lumilio/ decides a path's [Class], and each class
// has one [Policy] for creation, permission, registration, cleanup, and
// backup ([Policies]):
//
//	.lumilio/
//	├── user/        authored by people; irreplaceable; never rebuilt
//	├── trash/       deleted originals under retention
//	├── cache/       rebuildable derivatives; CACHEDIR.TAG
//	├── work/        0700; owned by one running operation
//	├── quarantine/  0700; bytes kept for a human decision
//	└── logs/        audit logs; size-rotated
//
// Anything else is [Unknown]: preserved unchanged and disclosed.
//
// # Paths
//
// A private [Path] comes only from a typed constructor ([Studio],
// [AssetSidecar], [Artifact], [TrashFile], [UploadSession], [Temp], ...) or,
// for a private path that was stored as text, from [Parse]. A media Path
// comes from [ParseMedia], which rejects the names [IsReserved] reports.
// No other package spells .lumilio; the Atlas rules private-path-spelling
// and private-path-construction enforce that.
//
// # Root names
//
// Outside .lumilio/ Lumilio creates only the markers and their locks,
// inbox/, and short-lived names starting with [TempPrefix].
package layout

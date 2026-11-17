package layout

import (
	"io/fs"
	"strings"
)

const (
	// PrivateRoot is the Repository's private tree. Its first segment
	// decides each path's class.
	PrivateRoot = ".lumilio"
	// TempPrefix starts every short-lived name Lumilio creates at a
	// Repository or Storage Location root: probes and marker temporaries.
	// Scan, the watcher, and recovery ignore it with one rule.
	TempPrefix = ".lumilio-tmp-"
)

// rootNames are the public files at a Repository or Storage Location root:
// the markers and their runtime locks. They are documented in the user
// manual; the marker packages read and write them.
var rootNames = map[string]bool{
	".lumiliorepo": true, ".lumiliorepo.lock": true,
	".lumilioroot": true, ".lumilioroot.lock": true,
}

// IsReserved reports whether a root-relative, slash-separated path belongs
// to Lumilio rather than to the user's media: the private tree, a marker or
// its lock, or a temporary name.
func IsReserved(rootRelative string) bool {
	first, _, _ := strings.Cut(rootRelative, "/")
	return first == PrivateRoot || rootNames[first] || strings.HasPrefix(first, TempPrefix)
}

// IsTemp reports whether a single name is a Lumilio temporary.
func IsTemp(name string) bool { return strings.HasPrefix(name, TempPrefix) }

// Class is one kind of private data. A path's class is its first segment
// under .lumilio/.
type Class string

const (
	// User is authored by people: Studio records and asset sidecars. It is
	// irreplaceable and never rebuilt.
	User Class = "user"
	// Trash holds deleted originals under retention, with their info.
	Trash Class = "trash"
	// Cache holds rebuildable derivatives; the whole class is safe to
	// delete and is marked with CACHEDIR.TAG.
	Cache Class = "cache"
	// Work belongs to one running operation and is never resumed.
	Work Class = "work"
	// Quarantine keeps bytes for a human decision: failed uploads and the
	// receipts of registrations with the bytes they moved.
	Quarantine Class = "quarantine"
	// Logs holds the Repository's audit logs.
	Logs Class = "logs"
	// Unknown is anything else, such as content a newer build wrote. It is
	// preserved unchanged and disclosed.
	Unknown Class = "unknown"
)

// Creation says when a class directory comes into existence.
type Creation string

const (
	CreatedAtInit Creation = "init"
	CreatedLazily Creation = "lazily"
)

// Registration is what open, adopt, and register-copy do with a class.
type Registration string

const (
	// KeepInPlace leaves the class where it is; User is reassociated only
	// on proven content.
	KeepInPlace Registration = "keep"
	// AdoptUnderHold adopts the class into the catalog under a hold.
	AdoptUnderHold Registration = "adopt"
	// ReuseWhenFenced keeps derivatives whose fences match, else rederives.
	ReuseWhenFenced Registration = "reuse"
	// MoveToQuarantine moves the bytes under a recovery receipt.
	MoveToQuarantine Registration = "quarantine"
)

// Cleanup is who may remove the class's data.
type Cleanup string

const (
	NeverAutomatic  Cleanup = "never"
	RetentionPolicy Cleanup = "retention"
	ArtifactCleaner Cleanup = "cleaner"
	OperationEnd    Cleanup = "operation-end"
	UserDecision    Cleanup = "user-decision"
	Rotation        Cleanup = "rotation"
)

// Backup is whether a backup of the Repository should include the class.
type Backup string

const (
	BackupInclude  Backup = "include"
	BackupExclude  Backup = "exclude"
	BackupOptional Backup = "optional"
)

// Policy is one row of the class table: how a class is created, protected,
// treated on registration, cleaned up, and backed up.
type Policy struct {
	Class        Class
	Created      Creation
	Perm         fs.FileMode
	Registration Registration
	Cleanup      Cleanup
	Backup       Backup
}

// Policies is the class table, in the order of the layout. It is the only
// declaration of the private tree's classes.
var Policies = []Policy{
	{Class: User, Created: CreatedAtInit, Perm: 0o755, Registration: KeepInPlace, Cleanup: NeverAutomatic, Backup: BackupInclude},
	{Class: Trash, Created: CreatedAtInit, Perm: 0o755, Registration: AdoptUnderHold, Cleanup: RetentionPolicy, Backup: BackupInclude},
	{Class: Cache, Created: CreatedAtInit, Perm: 0o755, Registration: ReuseWhenFenced, Cleanup: ArtifactCleaner, Backup: BackupExclude},
	{Class: Work, Created: CreatedAtInit, Perm: 0o700, Registration: MoveToQuarantine, Cleanup: OperationEnd, Backup: BackupExclude},
	{Class: Quarantine, Created: CreatedLazily, Perm: 0o700, Registration: KeepInPlace, Cleanup: UserDecision, Backup: BackupInclude},
	{Class: Logs, Created: CreatedAtInit, Perm: 0o755, Registration: KeepInPlace, Cleanup: Rotation, Backup: BackupOptional},
}

// PolicyOf returns the row of a class. Unknown has a row of its own: kept
// in place, never cleaned, included in backups.
func PolicyOf(class Class) Policy {
	for _, policy := range Policies {
		if policy.Class == class {
			return policy
		}
	}
	return Policy{Class: Unknown, Registration: KeepInPlace, Cleanup: NeverAutomatic, Backup: BackupInclude}
}

// Classify returns the class of a private Path from its first segment under
// .lumilio/. The private root itself, and any path that is not private, is
// Unknown.
func Classify(p Path) Class {
	if !p.IsPrivate() {
		return Unknown
	}
	rest, ok := strings.CutPrefix(p.value, PrivateRoot+"/")
	if !ok {
		return Unknown
	}
	first, _, _ := strings.Cut(rest, "/")
	class := Class(first)
	if PolicyOf(class).Class == Unknown {
		return Unknown
	}
	return class
}

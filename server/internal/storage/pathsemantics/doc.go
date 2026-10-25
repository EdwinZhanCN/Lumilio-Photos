// Package pathsemantics turns filesystem-specific name comparison rules into
// durable repository name keys. [Semantics] is the rule of one volume (case
// mode and Unicode normalization); [Semantics.NameKey] keys one component
// and [Semantics.PathKey] a '/'-separated repository-relative path.
//
// The rule is never assumed from the host: [Probe] measures it on a
// directory by creating hidden temporary files and looking them up under a
// case-changed and a normalization-changed spelling, and [Qualify] requires
// every directory of a tree to agree. A lookup that fails for any reason
// other than "no such file", or directories that disagree, is [ErrUnqualified]
// (or [ErrMixed]); the storage package stores the measured rule on the
// repository row and refuses to register or reconnect a tree it cannot
// qualify.
//
//atlas:group storage
package pathsemantics

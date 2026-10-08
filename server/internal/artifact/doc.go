// Package artifact owns the one immutable publication contract for derived
// pipeline files inside a registered repository. [Store.Publish] writes a
// derived file to an [Identity] made of the source fence, stage, pipeline
// version, and name before the catalog activates it. The first complete
// regular file at an identity is canonical: a retry adopts and hashes it even
// if a nondeterministic encoder would produce different valid bytes. No
// catalog row ever references a temporary or partial artifact; the [Cleaner]
// removes artifacts nothing references.
//
//atlas:group storage
package artifact

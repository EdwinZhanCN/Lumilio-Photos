// Package testfixture builds deterministic Storage Location and Repository
// trees for tests. It follows the shared internal/testutil convention: only
// tests import it; production code must not depend on these builders.
//
// [NewLocation] and [Location.Repository] use the portable marker writers.
// [NewLeftoverPrimary] adds readable Trash and Studio sidecars, abandoned
// staging, and unknown private data. [CopyRepository] preserves the source
// UUID and bytes. Every builder owns its tree through testing.TB.TempDir;
// names, identities, contents, and modification times do not depend on the
// temporary path, wall clock, or host mount classification.
//
//atlas:group storage
package testfixture

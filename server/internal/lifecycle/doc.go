// Package lifecycle holds the Asset lifecycle's catalog rules. An Asset
// exists if and only if it has at least one entry in repository_entries; its
// lifecycle state (active, missing, trashed) is derived from those entries by
// catalog triggers. [PurgeEntriesTx] is the Asset hard-delete boundary.
//
// Present or bound pending-hash entries win as active, then missing, then
// trashed. Offline describes availability, not a fourth lifecycle state;
// Missing and Trash retain metadata until their last entry is purged.
// [PurgeEntriesTx] removes entries and collects Assets with none left, including
// metadata and derived rows. [PurgeRepositoryEntriesTx] uses the same collection
// boundary for repository removal; copies in other repositories survive.
// Neither performs filesystem I/O; callers unlink trashed files first, and the
// artifact cleaner later reclaims artifacts whose Asset no longer exists.
//
//atlas:group domain
package lifecycle

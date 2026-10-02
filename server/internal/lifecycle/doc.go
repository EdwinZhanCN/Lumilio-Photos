// Package lifecycle holds the Asset lifecycle's catalog rules. An Asset
// exists if and only if it has at least one entry in repository_entries; its
// lifecycle state (active, missing, trashed) is derived from those entries by
// catalog triggers. PurgeEntriesTx is the only hard delete of an Asset.
//
//atlas:group domain
package lifecycle

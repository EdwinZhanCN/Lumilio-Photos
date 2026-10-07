// Package event owns deterministic media Event semantics: owner-scoped Event
// candidates, the events-v1 segmentation and reconciliation, user
// corrections, and resolution.
//
// Event topology is owner-wide and derived from logical media_item facts,
// never from individual files or repositories. The source and published
// revisions plus the shared [Resolver] (and [ResolveTx]) are the lifecycle
// authority; a repository Browse Scope is applied only as a read projection
// after owner authorization. [MarkEventFactsChangedTx] is the single factual
// invalidation boundary: any change that can move an Event calls it inside the
// same transaction.
//
// Owner scope is explicit at this boundary. An owner-scoped topology always
// carries its resolved owner into downstream asset queries; a nil owner never
// means "infer it". See .agents/decisions/2026-08-10-event-owner-topology.md.
//
// Events are delivered through the closed rebuild_projection_batch macro;
// River is not the lifecycle authority. source_revision > published_revision
// is pending rebuild work, and event_dirty_ranges is a recovery ledger, not an
// incremental computation window. Publish replaces an owner's complete
// membership in one revision-checked transaction, deleting before inserting.
// Manual corrections are exact logical-media assignments, and a command
// followed by two rebuilds is a fixed point. POST /api/v1/events/rebuild only
// enqueues work and answers 202 Accepted. Event shares and Agent references
// materialize immutable snapshots of displayable Assets, and automatic
// membership uses no ML signal.
//
// Events contain only photo/video logical media, including Live Photos;
// audio belongs in Music. Audio never seeds or bridges segmentation and cannot
// be manually added. Resolution excludes it from membership, counts, covers,
// Browse Scope, navigation, and share snapshots. Rebuild discards audio
// memberships, cover overrides, and constraints with audio endpoints while
// preserving eligible corrections; stale audio-only Events retire. See
// .agents/decisions/2026-10-06-audio-event-participation.md.
//
//atlas:group domain
package event

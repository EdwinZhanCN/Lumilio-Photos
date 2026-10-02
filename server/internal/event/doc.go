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
//atlas:group domain
package event

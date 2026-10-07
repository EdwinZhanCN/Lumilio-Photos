// Package facets computes ref.FacetSummary aggregates over a ref snapshot.
// It is shared by the describe tool (the agent's eyesight) and the hydration
// API (ref metadata for the frontend). All user-content strings are passed
// through ref.SanitizeUserText here, so callers can emit values as-is (INV-7).
//
//atlas:group agent
package facets

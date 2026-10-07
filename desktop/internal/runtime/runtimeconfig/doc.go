// Package runtimeconfig stores the complete, schema-versioned Server intent
// and its crash-recoverable current/LKG pointers. It never reads secrets or
// fills missing manifest fields; strict validation belongs to server/config.
//
// Onboarding materialises a complete candidate from the explicit desktop-local
// profile, substitutes OS-owned paths, and runs the same strict loader before
// exposing a small structured projection to the Settings window. Draft reads
// and patches never persist intent; [TransactionController.Save] and
// [TransactionController.Apply] are the only pointer-changing boundaries. Full
// TOML is an optional advanced recovery surface, not a first-run requirement.
//
// Apply journals prepared, stopping-previous, previous-stopped,
// candidate-selected, and committing phases; it proves the old generation
// released ownership before the candidate starts and promotes the candidate to
// last-known-good only after it is ready. Candidate edits are fingerprinted
// and guarded by aggregate version and base fingerprint.
//
//atlas:group desktop-runtime
package runtimeconfig

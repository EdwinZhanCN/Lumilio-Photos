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
//atlas:group desktop-runtime
package runtimeconfig

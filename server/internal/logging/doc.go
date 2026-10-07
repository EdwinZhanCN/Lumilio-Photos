// Package logging builds the process logger. [NewLogger] creates the zap-based
// [Runtime] from the manifest's logging section, [NewSlogZapHandler] and
// [RedirectStandardLog] route slog and the standard library logger into it,
// and [RepositoryAuditProvider] supplies per-repository audit logs for storage
// operations.
//
//atlas:group foundation
package logging

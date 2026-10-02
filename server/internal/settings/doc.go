// Package settings defines the runtime-mutable settings domain: the typed
// values whose single source of truth is the database `settings` table and
// which are changed at runtime through the API (Settings tabs + Setup), never
// through TOML. The immutable boot configuration lives in server/config.
//
// This package sits below both internal/service and internal/queue so that the
// MLConfigProvider interface (in queue) and the settings service (in service)
// can share these types without an import cycle. It depends on nothing internal.
//
//atlas:group foundation
package settings

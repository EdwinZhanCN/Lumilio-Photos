// Package storage adapts the in-process repository control handoff into the
// small, cache-backed surface needed by Tray and Settings. The cache is
// derived state: corruption is quarantined and never prevents Host startup.
//
//atlas:group desktop-runtime
package storage

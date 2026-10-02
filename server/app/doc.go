// Package app contains the server bootstrap: it wires configuration, logging,
// storage, the job queue, ML services, and the HTTP router, then serves until
// the provided context is cancelled. It is invoked by the CLI entrypoint
// (server/cmd) and imported in-process by the desktop supervisor, so it must
// own its full lifecycle (startup and graceful shutdown) without calling
// os.Exit. Fatal startup conditions are returned as errors for the caller to
// handle.
//
//atlas:group runtime
package app

// Package lumen installs and supervises the optional Lumen Hub child process.
// The [Controller] is independent of the Server generation: the Hub is a
// separately supervised process tree with its own owner lock, while all
// UI-facing state flows through the Desktop snapshot.
//
// A platform-specific, release-pinned Hub is installed from signed, staged
// artifacts into private app data. Before installation the Settings window
// offers the minimal, basic, and brave presets with the backends the platform
// supports and a model-cache directory; the Hub configuration is rendered from
// the same model as the upstream launcher and binds loopback
// [DefaultEndpoint], which the Desktop Server profiles name as a static node.
// Readiness comes from Lumen's versioned control gRPC service (a WatchStatus
// subscription), not a TCP probe, and logs are read with a bounded TailLogs
// request; model download and warm-up stay independent of Server readiness.
//
//atlas:group desktop-services
package lumen

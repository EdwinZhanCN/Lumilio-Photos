// Package control is the service surface the Desktop Settings window calls
// through Wails bindings. [DesktopService], [RuntimeService],
// [StorageService], [LumenService], [UpdateService], and [DiagnosticsService]
// validate requests and delegate to adapter interfaces implemented by the
// runtime, storage, Lumen, and update packages, so the bindings never reach a
// concrete controller directly.
//
// [PresentRuntime], [PresentLumen], and [ProjectCapabilities] derive the
// presentation (status dot, label, allowed actions) from a snapshot instead of
// storing UI state.
//
//atlas:group desktop-control
package control

// Package control is the service surface the Desktop Settings window and the
// tray call. [DesktopService], [RuntimeService], [StorageService],
// [LumenService], [UpdateService], and [DiagnosticsService] validate requests
// and delegate to adapter interfaces implemented by the runtime, storage,
// Lumen, and update packages, so the window never reaches a concrete
// controller directly.
//
// [PresentRuntime], [PresentLumen], and [ProjectCapabilities] derive the
// presentation (status dot, label, allowed actions) from a snapshot instead of
// storing UI state.
//
// The Settings window never calls the Server HTTP API and never takes part in
// product accounts, refresh cookies, CORS, or CSRF. It renders the typed
// Desktop snapshot the host hands it at each revision, so it can validate,
// apply, or restore runtime intent even when the Server fails to start. The
// product UI runs in the user's browser at http://localhost:6680, served by
// the in-process Server like any other.
//
//atlas:group desktop-control
package control

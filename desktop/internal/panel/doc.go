// Package panel is the Desktop Settings window, drawn as native UI with
// github.com/egoist/mygo/ui: there is no webview, HTML, or JavaScript.
//
// [Panel] holds the window's state and [Panel.View] builds its interface from
// it for every frame. The state is read and written on the UI thread only.
// The host hands [Panel.SetSnapshot] each revision of the control-plane
// snapshot, and the panel calls the control plane back through [Services],
// the same validating facade the tray uses, so it never reaches a concrete
// controller.
//
// Control-plane calls can block on a native dialog or a Server restart, so
// they run off the UI thread ([Options.Go]) and hand their result back
// through [Options.Post]. Tests run both inline and drive the view with
// ui.NewTester, without a window.
//
// Mutations acknowledge an operation before their actor work finishes. A
// tracker makes the snapshot's operation list, not the call's return,
// authoritative for a button's success and failure.
//
// Every string a person reads comes from [desktop/internal/locales]. The
// panel never calls the Server HTTP API and takes no part in product
// accounts: it can validate, apply, or restore runtime intent even when the
// Server fails to start.
package panel

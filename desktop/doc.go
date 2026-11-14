// Command desktop is the Lumilio Desktop App: a MyGo system-tray host that
// runs the complete Server in-process and opens the product in the system
// browser.
//
// main only assembles. It resolves OS paths, builds the state store and
// operation gate, constructs the runtime, storage, Lumen, update, and
// preferences controllers, and hands them to [desktop/internal/host], which
// runs the application: the tray, the native Settings window, and the
// coordinated quit. The hidden Lumen supervisor mode returns before any of
// that starts.
package main

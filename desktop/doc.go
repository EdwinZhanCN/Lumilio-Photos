// Command desktop is the Lumilio Desktop App: a Wails v3 system-tray host that
// runs the complete Server in-process and opens the product in the system
// browser.
//
// main only assembles services. It resolves OS paths, builds the state store
// and operation gate, constructs the runtime, storage, Lumen, update, and
// preferences controllers, registers the Wails services the private Settings
// window calls, and hands window and tray adapters to [desktop/internal/host].
// The Settings UI source is embedded so tests compile before the frontend is
// built.
//
//atlas:group desktop-host
package main

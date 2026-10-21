// Package shutdown is the only owner allowed to arm the Wails application for
// exit. Runtime and Lumen controllers are asked to quiesce through their typed
// interfaces; this package never creates processes or calls os.Exit.
//
//atlas:group desktop-runtime
package shutdown

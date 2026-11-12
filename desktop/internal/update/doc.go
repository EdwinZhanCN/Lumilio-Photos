// Package update owns the Desktop update aggregate: its snapshot, its
// operations, and the order check, download, apply.
//
// [Controller] never downloads or verifies anything itself. A [Provider]
// finds the newer [Release] of a channel, verifies what it downloads against
// the public key built into the app, and replaces the installed app; the
// host implements it with the framework's signed updater. A release whose
// signature the provider refuses ([ErrSignature]) never becomes applicable.
// Applying is a coordinated quit followed by a relaunch, which belongs to the
// shutdown coordinator.
//
//atlas:group desktop-services
package update

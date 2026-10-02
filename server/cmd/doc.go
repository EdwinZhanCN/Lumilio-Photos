// Command server is the standalone Server entry point.
//
// It requires --config with a complete runtime manifest, installs signal
// handling, applies the explicit break-glass environment whitelist, strictly
// loads the manifest, and calls [server/app.Run]. The `server config`
// subcommands initialise, validate, upgrade, and health-check manifests.
//
//atlas:group runtime
package main

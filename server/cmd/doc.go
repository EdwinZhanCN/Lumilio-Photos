// Command server is the standalone Server entry point.
//
// It requires --config with a complete runtime manifest, installs signal
// handling, applies the explicit break-glass environment whitelist, strictly
// loads the manifest, and calls [server/app.Run]. The `server config`
// subcommands initialise, validate, upgrade, and health-check manifests.
//
// Diagnostics are single-run flags: --pprof-addr, --agent-audit-log (the only
// place full prompts are captured), and --agent-ref-user-hot-budget-mib /
// --agent-ref-global-hot-budget-mib, which bound Agent reference hot memory
// (the global budget must be at least the per-user one). LUMILIO_BREAK_GLASS
// and LUMILIO_BREAK_GLASS_USERNAME are the only product environment controls;
// they are read here and passed separately from configuration. Desktop
// resource locations, test opt-ins, and container environment are host or
// harness contracts, never Server configuration.
//
//atlas:group runtime
package main

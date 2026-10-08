// Package config is the strict runtime-immutable configuration boundary.
//
// [LoadAppConfig] (and [LoadAppConfigBytes] for the Desktop) decodes one
// complete TOML manifest at [SchemaVersion] into [AppConfig]. It never
// searches for files, reads ordinary environment variables, or fills defaults:
// a missing, unknown, legacy, contradictory, or invalid field fails startup.
// Paths resolve relative to the manifest, secret values are read only from
// explicit secret files, and startup logs the absolute path, schema version,
// and source SHA-256 without secret content. The database section names the
// catalog path and a distinct queue_path; repository_scan sets the mandatory
// startup-and-periodic verification interval and the settle window.
//
// [GenerateManifest] renders a complete manifest for a named [Profile]
// (development, Desktop, Docker). [RenderExamples] produces the checked-in
// server/config/examples (a golden test forbids hand edits) and
// [GenerateJSONSchema] the editor schema, which covers presence, types, and
// closed value sets only; conditional legality stays in the loader. The Server
// never rewrites a manifest: an older supported version fails startup pointing
// at `server config upgrade`, which runs [UpgradeManifest], requires the
// result to pass the strict load, and keeps the original as a .bak file.
//
// Runtime-mutable settings — ML and LLM features, reverse geocoding — are not
// configuration; they live in the catalog (see [server/internal/settings]).
//
// repository_trash.retention_days is required and positive, with no code
// default; generated manifests write 30. It sets repository Trash retention
// before the runtime's hourly expiry pass.
//
//atlas:group foundation
package config

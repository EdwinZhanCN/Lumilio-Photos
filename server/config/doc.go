// Package config is the strict runtime-immutable configuration boundary.
//
// [LoadAppConfig] decodes one complete, schema-versioned TOML manifest into
// [AppConfig]. It never searches for files, reads ordinary environment
// variables, or fills defaults: a missing field is an error. Paths are
// resolved relative to the manifest, secret values are read only from explicit
// secret files, and the source bytes are fingerprinted.
//
// [GenerateManifest] renders a complete manifest for a named [Profile]
// (development, Desktop, Docker); [RenderExamples] produces the checked-in
// examples under server/config/examples and [GenerateJSONSchema] the editor
// schema. [UpgradeManifest] migrates older manifests explicitly.
//
// Runtime-mutable settings are not configuration; they live in the catalog
// (see [server/internal/settings]).
//
//atlas:group foundation
package config

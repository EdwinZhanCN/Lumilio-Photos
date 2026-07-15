# ADR 0001: Strict Runtime Configuration Manifest

- **Status:** Accepted
- **Date:** 2026-07-14
- **Decision owners:** Lumilio Photos maintainers
- **Decision type:** Breaking pre-release architecture decision

## Context

Lumilio Photos separates state into three non-overlapping domains:

1. **Frontend preferences:** browser localStorage, changed immediately by one client.
2. **Runtime mutable settings:** PostgreSQL is authoritative; Settings/Setup APIs modify them.
3. **Runtime immutable configuration:** injected before process start, read-only afterwards, and changed only by restarting.

First-run bootstrap is orthogonal to those domains. It is a state machine
(`fresh → db_rotated → admin_created → ready`) that observes setup gates; it is
not an additional configuration source.

The previous immutable configuration path composed several authorities:

```text
code defaults → TOML → environment overrides → derived paths → secret files
```

Consumers also supplied fallback values, the Lumen SDK applied its own defaults
and environment, some packages called `os.Getenv`, and desktop constructed a
typed config on a separate path. Consequently, TOML alone could not explain a
running process; omitted fields silently changed across versions; ambient env
could override deployments; and secret, ordinary configuration, and one-time
operator controls had unclear boundaries.

The project is pre-release, so compatibility with old manifests, env-only
deployments, and bootstrap volumes is deliberately not retained.

## Decision

### TOML is the runtime-immutable manifest

Every standalone launch supplies a complete manifest through:

```text
server --config <path> [--pprof-addr <addr>] [--agent-audit-log <path>]
```

TOML is not an override layer or a set of recommendations. After strict decode,
path resolution, validation, and secret resolution, it produces the only valid
`AppConfig`. Startup records the absolute manifest path, schema version, and
SHA-256 of the TOML bytes. Secret content is never included in that fingerprint
or log output.

### Runtime-immutable fields have no defaults

The config package does not construct development/production defaults, infer
missing fields from `environment`, search for config files, or let consumers
fallback on zero values. Example values are copyable recommendations only.

Code constants remain valid for protocol invariants, file-layout conventions,
and internal algorithms outside `AppConfig`. Runtime-mutable settings and
frontend preferences retain defaults owned by their own domains. Once a value
enters `AppConfig`, it must not also have a code fallback.

### Schema v1 is strict and complete

Every manifest declares:

```toml
schema_version = 1
```

The loader rejects unknown versions, fields or sections; missing fields; invalid
enum, duration, URL, origin, port, or `host:port`; contradictory combinations;
and unreadable required secret files. Pointer-backed decode types distinguish an
omitted field from an explicit false, zero, empty string, or empty array.

Relative paths resolve from the manifest directory, never the process working
directory. Bare media-tool commands such as `ffmpeg` retain PATH lookup
semantics. Explicit empty values are limited to:

- `server.web_root = ""` for API-only operation;
- `database.tools_bin_dir = ""` for version-matched PostgreSQL tool discovery;
- empty Lumen Hub URL/static nodes when another backend is configured;
- empty CORS origins;
- empty WebAuthn RP ID/origins in `origin-derived` mode.

All other strings, paths, commands, durations, and positive counts are required.

Schema v1 sections are `database`, `server`, `logging`, `storage`,
`repository_scan`, `geocoding`, `auth`, `transcode`, `lumen`, and `tools`. The
tracked canonical examples are:

- `server/config/server.example.toml` for local development;
- `server/config/server.container.toml` for the image;
- `desktop/supervisor/server.template.toml` for desktop compilation.

### Environment does not override AppConfig

Lumilio server accepts no ordinary runtime-config environment variables,
including config location, server/logging, database, storage, scanner,
geocoding, auth, transcode, Lumen, tools, secret paths, or secret values.

The only standalone product runtime env whitelist is:

- `LUMILIO_BREAK_GLASS`
- `LUMILIO_BREAK_GLASS_USERNAME`

The CLI host reads these once and passes them as `OperatorControls`; they never
modify `AppConfig`. Pprof and Agent audit are CLI flags. Desktop development
resource location, test/conformance/benchmark opt-ins, and environment required
by third-party containers or libraries are host/harness contracts rather than
Lumilio server configuration.

### Secrets are referenced files

TOML is authoritative for secret locations, never secret content. The complete
security-sensitive launch input is:

```text
TOML manifest + files referenced by the manifest
```

Database configuration declares `bootstrap_password_file` and
`rotated_password_file`. Bootstrap must exist, be readable, and be non-empty.
Rotated may be absent on first boot. Connection uses rotated when present and
bootstrap otherwise. Setup writes a newly rotated password with mode `0600`.
Self-heal uses bootstrap if a database volume was reset while a rotated file
survived. `auth.secret_key_file` must be explicit; the app may create the file
at that path with mode `0600`, but may not choose a default or read it from env.

### Desktop compiles and consumes the same manifest

Desktop is a manifest compiler, not a second config constructor:

```text
desktop-settings.json + resource discovery
  → version-controlled template (missingkey=error, TOML literal encoding)
  → atomic app-data/config/server.toml write (0600)
  → LoadAppConfig(server.toml)
  → app.Run
```

Stable policy lives in the template; machine bindings include port, SPA/log/
storage paths, database socket and identity, secret files, bundled tools,
WebAuthn origin, and the local Lumen endpoint. Write or reload failure blocks
startup. The generated file is reconstructed per launch but is the actual
immutable input for that process; persisted user choices remain in
`desktop-settings.json`.

### Dependencies cannot bypass the manifest

Photos constructs the Lumen SDK config directly. Every discovery, timeout,
backoff, scan, and chunk field Photos consumes is explicitly mapped from
`[lumen]`; SDK `DefaultConfig`, `LoadFromEnv`, and unrelated broker-server/
logging validation are not used.

Any future dependency option affecting runtime-immutable behavior must either
be mapped explicitly into the manifest or documented as a non-configurable
protocol/implementation invariant.

### Construction boundary

Production configuration has one legal entry: `LoadAppConfig(path)`. Business
packages must not load TOML, read config env, construct defaults, mutate global
config, or fallback on empty values. `app.Run` rejects an `AppConfig` not marked
as loader-produced and remains the composition root that distributes narrow
resolved values.

## Consequences

Positive consequences:

- effective immutable behavior is auditable, reproducible, and manifest-diffable;
- new required fields fail explicitly instead of silently changing old deployments;
- standalone, container, and desktop share decode and validation;
- ambient environment cannot alter behavior;
- secret lifecycle and operator controls have explicit boundaries;
- consumers and the Lumen adapter no longer implement precedence/fallback logic.

Accepted costs:

- manifests are longer and every field must be maintained explicitly;
- adding an immutable field requires updating every template and test;
- desktop manifest output becomes startup-critical;
- container deployments must provision secret files;
- old TOML, env-only deployments, and old bootstrap volumes do not work;
- temporary port/log/database changes require another manifest, not an env flag;
- tests need complete fixtures rather than zero-value config.

## Alternatives considered

- **Code defaults + TOML + env precedence:** rejected because it retains several authorities.
- **TOML primary with env overrides:** rejected because TOML remains non-auditable.
- **TOML only for standalone:** rejected because desktop would retain a second validation path.
- **Generate desktop TOML without reloading it:** rejected because the file would remain a projection rather than the proven input.
- **Secret environment variables:** rejected due to inheritance, process-diagnostic exposure, and weaker source auditability.
- **Expose every code constant:** rejected because this decision governs runtime-immutable configuration, not protocol and algorithm internals.

## Migration

This is a no-compatibility migration:

1. Introduce strict schema v1 and complete fixtures.
2. Delete defaults, env overlays, `.env` loading, and automatic file search.
3. Remove consumer fallbacks and map Lumen directly.
4. Switch CLI, local, container, and desktop startup to complete manifests.
5. Split bootstrap/rotated/app-key secret file contracts.
6. Delete old env templates and typed desktop/debug-copy behavior.
7. Run `make dev-reset` for local development, then rebuild bootstrap state.
8. Require deployed users to create a new manifest and secret files.

Old fields intentionally fail as unknown; there is no automatic translator.

## Compliance

Every future runtime-immutable field must update the schema version (or the
still-unreleased v1), all three templates, missing/invalid/mapping tests,
operator-visible runtime info when relevant, and this architecture record. It
must not receive a code fallback or env override.

If a value must change through an API while running, it belongs in runtime
mutable settings. If it affects only one browser user, it belongs in frontend
preferences.

# Runtime-Mutable Reverse Geocoding

Status: completed on 2026-08-13. Target contracts were frozen on 2026-08-13.

Primary owners: `server/internal/settings`,
`server/internal/service/settings_service.go`,
`server/internal/service/location_service.go`,
`server/internal/queue`, `server/migrations`, `server/config`, and
`web/src/features/settings/flows/server`.

## Goal

Move reverse-geocoding configuration out of the runtime-immutable TOML
manifest and into the singleton SQLite `settings` row. An administrator must be
able to change the effective provider, endpoint, response language, and HTTP
identity from `/settings?tab=server` without restarting the Server or Desktop
runtime.

The primary user journey is:

1. Open Settings → Server.
2. Find Reverse geocoding after Health check interval and before Database
   backups.
3. Select `Disabled` or `Nominatim`, edit the provider fields, and explicitly
   save the draft.
4. Observe the saved values immediately on refresh and in every runtime
   generation backed by that catalog.
5. When enabled, let a durable background resolver update existing and future
   location clusters without rebuilding their membership and without using a
   Server restart as a correctness mechanism.

`disabled` remains the default. Enabling a remote endpoint is an explicit
admin-controlled privacy decision because photo coordinates leave the local
instance.

## Current problem

Reverse geocoding is currently classified as boot configuration even though it
does not own a listener, database connection, filesystem location, secret, or
other process-lifetime resource:

- schema-v3 TOML requires `[geocoding]` with `provider`,
  `nominatim_endpoint`, `language`, and `user_agent`;
- `server/app` passes `config.GeocodingConfig` into `NewLocationService` once,
  and the service constructs one boot-lifetime `ReverseGeocoder`;
- Settings → Server reports only `geocoding_provider` inside the read-only
  runtime-info projection;
- changing the effective behavior therefore requires editing TOML and
  restarting, including in Desktop where TOML is an advanced host surface.

The resolver also has state contracts that make a mechanical four-column move
incorrect:

- only `pending` clusters are selected, while disabling converts pending rows
  to `disabled`; enabling later cannot resolve them without a full rebuild;
- a location rebuild deletes and recreates cluster identity and membership
  before performing remote HTTP inline;
- one invocation reads at most 500 pending rows and does not durably continue
  beyond that batch;
- cache identity is `provider + language + geohash`, so changing the endpoint
  can reuse a result from a different source; and
- no settings revision prevents an in-flight request made under old settings
  from publishing after an administrator disables or changes the provider.

## Fixed decisions

### Ownership and scope

- The SQLite `settings` row is the only source of truth for reverse-geocoding
  configuration. TOML, environment variables, Desktop host settings, and code
  fallbacks must not become secondary override layers.
- The complete mutable aggregate is global and admin-owned:
  `provider`, `nominatim_endpoint`, `language`, and `user_agent`.
- `language` is instance-wide in this plan. Per-user or per-owner localized
  place labels require a different storage model and are out of scope.
- The provider vocabulary remains exactly `disabled | nominatim`.
- Stored defaults are:
  - provider: `disabled`;
  - endpoint: `https://nominatim.openstreetmap.org/reverse`;
  - language: `en`;
  - user agent: `Lumilio-Photos/1.0`.
- All four fields remain stored and valid while the provider is disabled so an
  administrator can prepare a configuration before enabling it.
- Saving settings validates syntax only. It does not make a remote probe part
  of the PATCH request or hold an HTTP request open while clusters resolve.

### Destructive-only reset

There is no production catalog and no compatibility obligation.

- Create a new complete SQLite schema-generation baseline at migration version
  `000008`; it is the first migration of schema generation 6 and contains the
  final schema represented today by baseline `000003` plus migrations
  `000004`–`000007`, together with this plan's geocoding changes.
- Set `schemaGeneration = 6`,
  `currentGenerationBaselineVersion = 8`, and `PRAGMA user_version = 6`.
- Do not write an upgrade migration from generation 5, copy old settings,
  inspect old TOML, or retain a compatibility view/column/query. Generation-5
  catalogs fail the existing generation check and instruct the operator to
  delete the catalog.
- Historical migration files remain immutable audit records but are not read
  for a generation-6 catalog. `sqlc.yaml` reads the new complete baseline, not
  the old generation.
- Local developers reset application state with `task dev:reset`; repository
  media storage and original files remain preserved. No automatic reset runs
  at application startup.
- Increment the runtime manifest to schema v4 and remove `[geocoding]`
  completely. A schema-v3 manifest is rejected; there is no manifest rewrite,
  legacy decoder, ignored legacy section, or one-shot TOML import.

### Persistence contract

The generation-6 `settings` table adds:

| Column | Contract |
| --- | --- |
| `geocoding_provider` | non-null `disabled | nominatim`, default `disabled` |
| `geocoding_nominatim_endpoint` | non-empty absolute HTTP(S) URL |
| `geocoding_language` | non-empty normalized response-language value |
| `geocoding_user_agent` | non-empty HTTP User-Agent value |
| `geocoding_revision` | positive integer, incremented only when the normalized geocoding aggregate changes |

The same baseline adds durable retry state to `location_clusters`:

| Column | Contract |
| --- | --- |
| `geocode_attempt_count` | non-negative attempts for the current pending projection |
| `geocode_next_attempt_at` | nullable UTC retry time; null for immediately eligible or terminal rows |

`reverse_geocode_cache` gains a non-empty `source_key`. Its uniqueness and
lookup contract become `(source_key, geohash)`; the old provider/language-only
cache identity does not survive the destructive baseline.

`server/internal/settings` owns a typed `Geocoding` value and its normalization,
validation, defaults, source identity, and provider predicates. The service,
queue, and HTTP adapter consume this type; `server/config` no longer defines a
geocoding type.

The normalized endpoint must:

- be absolute and use `http` or `https`;
- have a non-empty host;
- reject embedded credentials and fragments; and
- remain capable of addressing loopback or LAN-hosted Nominatim because an
  authenticated administrator is explicitly allowed to choose a local
  provider.

Endpoint is limited to 2,048 bytes, language to 64 bytes, and User-Agent to 512
bytes. User-Agent also rejects CR, LF, and other invalid HTTP header bytes.
Enforce the same limits at the service/API boundary and with database `CHECK`
constraints. Validation failures are HTTP 400 responses and do not modify any
part of the settings row.

### Revision and publication contract

- Every normalized aggregate change increments `geocoding_revision` in the
  same transaction as the settings write.
- Resolver jobs carry the expected revision. A stale job exits successfully
  without making another remote request.
- Before every cache lookup or remote request, the resolver confirms that the
  job revision is still current. Cache identity remains source-based rather
  than revision-based. Every cluster-result or retry-state update publishes
  only while the singleton settings row still has the job revision. A
  configuration change racing between request start and response publication
  therefore makes the old result a no-op.
- Settings persistence, any required cluster state reset, and insertion of the
  resolver job share one SQLite transaction through River `InsertTx`. If job
  insertion fails, the settings update rolls back and the UI receives an error.
- The settings service receives the database/queue dependencies needed for
  this transaction. Reorder application assembly so the River client exists
  before constructing the final settings service; do not add a mutable
  post-construction queue setter.

Result identity is distinct from execution revision:

```text
source_key = SHA-256(provider + canonical endpoint + normalized language)
```

`user_agent` is deliberately excluded because it changes request identity, not
the provider's place-name result. Cache identity becomes `source_key + geohash`;
changing endpoint or language cannot reuse another source's response, while
disabling and later re-enabling the exact same source may reuse an unexpired
cache entry.

### State transitions

Changing settings must not delete or rebuild `location_clusters` or
`location_cluster_assets`.

| Transition | Cluster projection | Background action |
| --- | --- | --- |
| enabled → disabled | stop new calls; convert unresolved `pending`/`failed` rows to `disabled`; retain already resolved labels and cache rows; clear retry counters/times | stale revisions self-cancel; no new resolver job |
| disabled → nominatim | clear derived label/source fields, reset retry state, and set all clusters to `pending` | enqueue resolver at the new revision |
| enabled provider, endpoint, or language changes | clear derived label/source fields, reset retry state, and set all clusters to `pending` | enqueue resolver at the new revision |
| only User-Agent changes while enabled | retain resolved labels; reset retry state and return `disabled`/`failed` rows to `pending` | enqueue resolver at the new revision |
| any unchanged normalized PATCH | no revision bump and no location work | none |

Disabling does not erase previously derived place names, raw cached provider
responses, or original GPS metadata. A separate destructive “clear geocoding
data” action is not part of this plan.

New cluster creation reads settings inside the same SQLite transaction that
writes the cluster topology. It creates `disabled` rows without a resolver job
when the provider is disabled, or `pending` rows plus a revisioned resolver job
when enabled. This serialization prevents a rebuild that began near a settings
change from publishing work for the wrong revision.

## Resolver architecture

Remote reverse geocoding moves out of `RebuildLocationClusters` into one
durable boundary:

```text
metadata/manual request
        |
        v
rebuild_location_clusters (SQLite topology only)
        |
        v
resolve_location_clusters { geocoding_revision }
        |
        +--> current settings snapshot + revision guard
        +--> source-aware cache
        +--> paced Nominatim HTTP adapter
        +--> revision-checked cluster projection
```

- Add `ResolveLocationClustersArgs{GeocodingRevision}` and its worker. Run it on
  the existing single-worker location queue so topology rebuild and label
  resolution remain serialized without introducing another queue owner.
- Deduplicate jobs by revision across pending, scheduled, retryable, and running
  states. A newer revision may coexist long enough for the older job to notice
  staleness and exit.
- Process at most 25 eligible clusters per worker invocation. When immediately
  eligible or future-scheduled pending rows remain, snooze the same River job
  until the next eligible time; do not complete it and hope another event
  inserts a successor. This replaces the current one-shot 500-row ceiling.
  Do not keep a database transaction open across remote HTTP.
- The provider adapter owns a conservative single-request concurrency limit and
  a minimum one-second interval between Nominatim requests. These safety limits
  are not user settings in this plan.
- Respect `Retry-After` on 429 responses. Timeouts, connection failures, 429,
  and 5xx responses update the cluster's durable attempt count/next-at time and
  allow the batch to continue. Use at most eight provider attempts with
  exponential delay bounded from 5 seconds to 5 minutes; a valid `Retry-After`
  may extend the delay up to one hour. Mark exhaustion and provider-declared
  permanent failures as `failed`. Database/queue failures remain River job
  errors and use River's bounded job retry rather than provider retry state.
- Keep the existing 10-second per-request timeout and 30-day cache TTL unless
  focused tests justify changing them. Bound response bytes before JSON decode.
- Never log coordinates, response bodies, or a URL after latitude/longitude
  query parameters have been added. Diagnostic logs may include provider,
  source-key prefix, revision, status class, and cluster id.
- `RebuildLocationClusters` remains responsible only for deterministic geohash
  grouping/membership and transactional resolver enqueue. Listing location
  clusters remains a read-only projection.

## API contract

Extend the existing admin-only system-settings API; do not create a second
geocoding settings endpoint.

`GET /api/v1/settings/system` adds:

```json
{
  "geocoding": {
    "provider": "disabled",
    "nominatim_endpoint": "https://nominatim.openstreetmap.org/reverse",
    "language": "en",
    "user_agent": "Lumilio-Photos/1.0"
  }
}
```

`PATCH /api/v1/settings/system` accepts the same nested aggregate as a partial
patch. Omitted fields retain their persisted values. Provider uses the explicit
wire values `disabled | nominatim`; do not introduce a UI-only sentinel that
must be translated by an untyped cast.

- Add typed service, DTO, mapper, partial-input, and sqlc fields.
- Do not expose `geocoding_revision`; it is an internal concurrency contract.
- Remove `geocoding_provider` from `RuntimeInfoDTO` because geocoding is no
  longer runtime-immutable.
- Preserve OpenAPI-first generation: update Go DTOs/annotations, run
  `task dto`, and never hand-edit `schema.d.ts` or generated Swagger/ReDoc.
- The PATCH response returns the committed effective settings. Background
  resolution is not represented as synchronous completion or a fake progress
  value.

## Settings → Server UI contract

Add a server-owned `GeocodingSection` and local draft hook under
`web/src/features/settings/flows/server`. The visible section order in
`ServerTab` is fixed:

1. Browser security notice;
2. Health check interval;
3. Reverse geocoding;
4. Database backups;
5. Runtime configuration.

The section contains:

- a provider dropdown with `Disabled` and `Nominatim`;
- an endpoint field;
- a response-language text control with `en` and `zh` suggestions, while
  preserving any other valid server-confirmed language value;
- a User-Agent field;
- concise privacy copy explaining that enabling the provider sends photo
  coordinates to the configured endpoint; and
- explicit Save and Reset behavior using the existing draft/settings-save
  primitives. Typing in endpoint or User-Agent fields must not autosave.

Nominatim-specific fields may be visually de-emphasized while disabled but
must remain readable and must retain their draft values. Saving `disabled`
must never trigger a validation request to the remote endpoint.

The section represents query loading/error state, disables duplicate submits,
shows mutation failure without discarding the draft, and resets to the last
server-confirmed values after success. Backup controls below it remain
independently usable.

Remove geocoding from the read-only runtime field list and update its copy so
the page does not claim that this setting requires TOML plus restart. Update
`settings/doc.ts`, regenerate its sibling `doc.md`, and follow extract-then-fill
for all new i18n keys.

## Non-goals

- Do not preserve or import values from schema-v3 TOML or generation-5 SQLite.
- Do not add environment overrides, Desktop-only mirrors, or ordinary code
  fallbacks.
- Do not add Google, Mapbox, AMap, offline gazetteers, API-key providers, or a
  generic provider-plugin registry.
- Do not make response language user-scoped.
- Do not add per-provider rate-limit knobs, cache TTL controls, a connection
  test endpoint, resolver progress UI, or a cache/data deletion UI.
- Do not change GPS extraction, geohash precision, location-cluster membership,
  Places/Map browsing semantics, or original media metadata.
- Do not call the public Nominatim service from automated tests.

## Execution phases

### Phase 1 — Cut the destructive schema and manifest generations

1. Build `000008` as a complete generation-6 baseline and fold the current
   final schema plus new settings/cache contracts into it.
2. Advance the database generation/baseline constants and baseline tests;
   point sqlc at the new baseline and regenerate repository code.
3. Advance runtime manifest schema to v4; delete geocoding from `AppConfig`,
   manifest structs, presence/semantic validation, enums, profiles, tests, JSON
   Schema, and generated example TOMLs.
4. Remove the dev-only `zh` geocoding profile mutation. Database seed values,
   not deployment profiles, now own the initial aggregate.

Exit: a clean database and schema-v4 manifest boot; a generation-5 catalog and
schema-v3 manifest fail explicitly; neither configuration source contains
geocoding.

### Phase 2 — Make settings and side effects atomic

1. Add `settings.Geocoding` normalization, validation, defaults, revision, and
   source-key behavior.
2. Extend the singleton table queries, service model, DTOs, and PATCH mapping.
3. Add transactional location reset/disable queries and revision-checked result
   publication.
4. Move River client construction early enough to inject the database and queue
   into the settings service; make settings write, cluster transition, and
   resolver `InsertTx` one commit.
5. Map typed configuration validation to HTTP 400 and operational/transaction
   failures to HTTP 500.

Exit: settings changes are visible without restart, unchanged normalized
patches are no-ops, and a queue insertion failure cannot leave a committed
configuration without its required reconciliation.

### Phase 3 — Separate topology from remote resolution

1. Add the revisioned resolver job/worker and register it on the location queue.
2. Make location rebuild read mutable settings transactionally, write only
   cluster topology, and enqueue resolution instead of performing HTTP inline.
3. Rework pending selection into bounded durable batches with stale-revision
   checks before request and before publication.
4. Add source-aware cache keys, conservative pacing, retry/backoff handling,
   response bounds, and safe diagnostic logging.
5. Preserve the exact disable/enable/provider/language/endpoint/User-Agent
   transitions frozen above.

Exit: libraries larger than 500 clusters drain without another rebuild; old
configuration cannot publish after a settings change; disabled mode makes zero
remote calls.

### Phase 4 — Add Settings → Server editing

1. Extend the generated settings types and feature API hook with the nested
   geocoding aggregate without response casts.
2. Add the draft hook and `GeocodingSection`, then place it immediately before
   `BackupSection` in `ServerTab`.
3. Remove the runtime-info geocoding row and add accessible loading, error,
   dirty, reset, save, and success behavior.
4. Extract i18n keys, fill Chinese values, update `settings/doc.ts`, and
   regenerate `settings/doc.md`.

Exit: an admin can configure, save, refresh, disable, and re-enable reverse
geocoding from `/settings?tab=server`; backup controls retain their existing
behavior and position below the new section.

### Phase 5 — Regenerate contracts and close documentation

1. Regenerate sqlc, config schema/examples, OpenAPI, TypeScript types, API docs,
   i18n catalogs, and feature documentation with their canonical commands.
2. Update `architecture.md`, `BACKEND.md`, configuration prose, and any Server
   Settings documentation to classify geocoding as SQLite-owned mutable state.
3. Run all validation boundaries below and record evidence in Progress.
4. Move this plan to `completed/`, retaining the goal, final contracts,
   destructive reset boundary, validation evidence, and durable decisions.

## Validation boundaries

### Backend invariants

- Fresh settings contain the exact disabled/default aggregate.
- Valid partial PATCHes preserve omitted fields; invalid provider/URL/language/
  User-Agent input returns 400 and changes nothing.
- Normalization-equivalent input does not increment revision or enqueue work.
- Enabling, disabling, result-source changes, and User-Agent-only changes apply
  the exact transition table above.
- A forced River `InsertTx` failure rolls back settings and cluster state.
- A stale job performs no new request and cannot update a cluster, including a
  response that races with a settings revision change.
- Endpoint or language changes cannot hit a cache entry from the prior source;
  disable/re-enable of the same source can use its unexpired cache.
- Disabled mode and disabled cluster creation make zero HTTP calls.
- A mock Nominatim endpoint receives the configured language and User-Agent;
  automated tests never contact a public service.
- More than 500 pending clusters eventually drain through bounded repeated work.
- 429/`Retry-After`, transient 5xx/network failure, permanent failure, timeout,
  and oversized response paths have deterministic status/retry behavior.
- Location rebuild preserves deterministic topology and transactionally queues
  resolution without performing HTTP itself.
- Generation-5 catalogs and schema-v3 manifests are rejected; clean
  generation-6/schema-v4 startup passes.

### Frontend scenarios

Use a Settings flow integration spec with MSW and generated DTO fixtures to
cover:

- the exact section order with Reverse geocoding before Database backups;
- loading and server-error states;
- hydrating the draft from `GET /settings/system`;
- dirty/reset behavior and explicit save;
- the exact nested PATCH body for disabled and Nominatim configurations;
- controls disabled during submission, error retention, and success reset; and
- no runtime-info geocoding row after the mutable section is introduced.

Resolve accessible names through `@test/i18n`; do not use copy literals or mock
TanStack Query hooks. A full browser E2E is not required unless implementation
changes a production-only browser boundary.

### Generated commands and repository gates

Run from the repository root unless a command states otherwise:

```text
task server:sqlc
task config:examples
task dto
cd web && vp exec i18next-cli extract
cd web && vp exec i18next-cli status
cd web && vp node --input-type=module -e '
  import { parseDocFile, renderMarkdown } from "@edwinzhancn/docts";
  import { writeFileSync } from "node:fs";
  const f = process.argv[1];
  writeFileSync(f.replace(/\.ts$/, ".md"), renderMarkdown(parseDocFile(f)));
' src/features/settings/doc.ts
task ci:architecture
task server:test
task web:test
task desktop:test
task ci:site
task test
```

Review generated diffs rather than hand-editing `schema.d.ts`, Swagger/ReDoc,
config examples/schema, translation key structure, sqlc output, or `doc.md`.

## Progress

- [x] Current TOML/settings/location/cache/UI ownership audited.
- [x] Runtime-mutable aggregate, destructive reset, UI placement, and resolver
  contracts frozen.
- [x] Phase 1 — Destructive schema and manifest generations.
- [x] Phase 2 — Transactional settings and side effects.
- [x] Phase 3 — Durable revisioned resolver.
- [x] Phase 4 — Settings → Server UI.
- [x] Phase 5 — Generated artifacts, documentation, and gates.

## Completion evidence

- `000008_geocoding_runtime_settings_baseline.up.sql` is the generation-6
  complete baseline; the runtime manifest is schema v4 with no `[geocoding]`
  section. Generation and manifest rejection tests remain in place.
- Settings normalization, source-key identity, revision transitions,
  transactional cluster resets, and River `InsertTx` rollback are covered by
  focused service/settings tests. The resolver adapter covers configured
  language/User-Agent, pacing, retry-after parsing, retry bounds, and response
  size limits without contacting a public service.
- `task server:sqlc`, `task config:examples`, `task dto`, i18n extract/status,
  and Settings feature-doc regeneration completed successfully.
- `task server:test` passed all Server packages, `task web:test` passed 85 test
  files (356 tests, 2 skipped files and 6 skipped tests), and `task ci:site`
  passed documentation checks/build. `git diff --check` passed.
- `task ci:architecture` and the root `task test` are still stopped by the
  pre-existing retired terminology phrase in
  `web/src/features/assets/doc.ts` and its generated `doc.md`; those files are
  outside this change. `task desktop:test` is stopped before the affected
  packages by the pre-existing dirty `desktop/go.mod`/`desktop/go.sum` change
  that selects `lumen-sdk` v1.4.1 without its package checksum; those user
  changes were preserved.

## Durable decisions

- Runtime geocoding has one SQLite-owned aggregate and one revisioned resolver
  job. Source identity excludes User-Agent, while publication is guarded by
  revision so endpoint/language changes cannot reuse or publish stale results.
- Location topology rebuilds never perform remote HTTP. Resolver work is
  bounded, paced, retryable through durable cluster state, and runs on the
  existing single-worker location queue.

## Decision log

- 2026-08-13: Reverse geocoding is SQLite-owned runtime-mutable state because it
  is admin-controlled product/privacy behavior, not a process-lifetime resource.
- 2026-08-13: All four existing fields move together; TOML retains no fallback
  or override.
- 2026-08-13: Database generation 6 and manifest schema v4 are destructive-only;
  no SQLite upgrade, TOML import, compatibility decoder, or dual-read period is
  allowed.
- 2026-08-13: The UI lives at `/settings?tab=server`, immediately before
  Database backups.
- 2026-08-13: Location topology rebuild and external place resolution are
  separate jobs; settings persistence and required resolver enqueue are one
  SQLite/River transaction.
- 2026-08-13: Disabling stops future requests but preserves already resolved
  labels and cached responses; deletion is a separate future product decision.

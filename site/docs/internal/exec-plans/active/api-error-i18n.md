# API Error Internationalization

Status: active. An initial cross-layer survey and the target contracts below
were frozen on 2026-08-17; implementation and the exhaustive inventory have not
started.

Primary owners: `server/internal/api`, `server/internal/api/handler`,
`server/internal/api/dto`, `web/src/lib/http-commons`, and the Web flows that
surface request or asynchronous-operation failures.

The current standard error response carries an HTTP status in `code`, an
English display sentence in `message`, and `err.Error()` in `error`. The Web
then recovers those fields through repeated local casts and often renders them
directly. This makes the API response itself the accidental copy authority,
prevents a language change from consistently translating failures, and exposes
internal implementation detail at the public HTTP boundary. There are also
typed-but-separate failure surfaces for Repository conflicts, Agent and upload
SSE, batch upload items, scan runs, host actions, backup restores, and cloud
operations.

## Goal

Make every user-facing API failure language-neutral on the wire and localized
by the Web client from a stable, typed error code. Internal causes remain useful
in structured Server logs but never become browser copy.

The work is complete only when:

- every non-2xx JSON API response exposes a stable error code rather than an
  English sentence or raw Go error;
- the same wire response renders appropriate English or Simplified Chinese
  copy using the current UI language, including after a runtime language
  change;
- streaming and asynchronous failures intended for ordinary users follow the
  same code-and-parameters model;
- unknown, legacy, network, and version-skewed errors fall back to localized
  operation copy without displaying a wire payload;
- OpenAPI and generated TypeScript describe the error contract, and Web code
  consumes those generated types without casts; and
- deterministic gates prevent new raw errors, handler-authored display copy,
  untranslated codes, or ad hoc Web parsers from entering the same paths.

## Non-goals

- Do not translate inside the Server or select response text from
  `Accept-Language`. The Server owns semantics; the UI owns presentation.
- Do not localize logs, support bundles, audit records, filenames, provider
  payloads, LLM output, or explicitly labeled administrator diagnostic detail.
- Do not turn success responses into a general message-localization project.
  A success DTO touched by this migration may stop carrying unused display
  copy, but success-copy cleanup is not an expansion criterion.
- Do not expose filesystem paths, SQL/library errors, credentials, tokens,
  provider responses, or arbitrary user input as translation parameters.
- Do not use this work to redesign domain behavior, HTTP status selection,
  authentication refresh, Repository recovery actions, or background-job
  state machines.
- Do not rename existing database, on-disk, HTTP resource, or generated
  identifiers solely to match product copy. Human-facing terminology still
  follows the canonical bilingual terminology registry.

## Failure-surface taxonomy

The migration distinguishes three contracts instead of treating every field
named `error` as the same thing.

### Request failures

Any non-2xx response under `/api/v1` that produces JSON uses the standard error
envelope or a typed extension of it. This includes failures produced before a
handler runs: request-origin policy, authentication/authorization, CSRF,
rate-limit, and application-initialization middleware. A media/download route
may return binary content on success, but its JSON failure still follows this
contract and declares that JSON response in OpenAPI.

### User-facing operation failures

An already-accepted operation can fail after the request succeeds. Agent and
upload SSE events, batch-upload item results, upload materialization status,
Repository scan status, native host actions, backup restore state, and cloud
operation state use stable error codes and bounded translation parameters when
their failure is shown as ordinary UI copy. Persisted codes remain stable
across restarts and language changes.

### Operator diagnostics

Monitor queue samples, storage support bundles, audit history, and structured
logs may retain sanitized technical detail because diagnosis is their explicit
job. The primary label for a known state is still localized from its stable
code. Diagnostic text must be visually and structurally separate, must not be
the only explanation presented to an ordinary user, and must never be copied
into the standard request-error envelope.

Existing domain status codes such as Event rebuild or Lumen discovery
diagnostics retain their owning plan and semantics. This plan supplies a shared
translation mechanism; it does not collapse those states into HTTP failures.

## Fixed wire contract

### Standard response

The public JSON shape is:

```json
{
  "code": 401,
  "error_code": "auth_invalid_credentials",
  "params": {}
}
```

- `code` remains the numeric HTTP status for compatibility and always equals
  the actual response status.
- `error_code` is required and is the presentation-independent semantic code.
- `params` is optional. It is a string-to-string object because its only job is
  bounded interpolation; it is not an escape hatch for arbitrary detail.
- `message` and `error` are removed from the public error response. In
  particular, there is no English fallback sentence in the wire contract.
- Headers remain authoritative for transport policy such as `Retry-After`.
  Such a header may have an allowlisted parameter mirror only when the UI needs
  that value for localized copy.

The type and all documented error responses remain OpenAPI-first. `task dto`
regenerates Server OpenAPI, the generated Web schema, and ReDoc. The generated
schema must expose a required string error-code type; the Web must not create a
parallel handwritten response type.

### Codes and parameters

- Codes use lowercase `snake_case`. Domain-specific codes carry a domain
  prefix, for example `auth_invalid_credentials`,
  `repository_unavailable`, or `upload_processing_failed`.
- Generic codes are deliberately small and stable:
  `request_invalid`, `authentication_required`, `permission_denied`,
  `resource_not_found`, `conflict`, `rate_limited`, `service_unavailable`, and
  `internal_error`.
- A new code represents a user-distinguishable recovery or explanation, not a
  unique call site or a rewritten English sentence. The operation-local Web
  fallback supplies context for generic failures.
- Each code has exactly one normal HTTP status and an allowlist of parameter
  names. A Server-side registry validates both before writing a response.
- Parameter values are bounded, display-safe facts such as a count, limit, or
  field name. Paths, raw causes, opaque provider messages, and unbounded values
  are forbidden even if a translation currently ignores them.
- Authentication codes preserve anti-enumeration behavior. Codes and
  parameters may not reveal whether an account, passkey, MFA method, refresh
  cookie, or recovery code exists when the current response intentionally
  makes those cases indistinguishable.

### Typed extensions

A response that needs recovery facts keeps its typed fields and adopts the
same `code` plus `error_code` semantics. For example,
`RepositoryConflictDTO` keeps `conflict_type`, Repository identity, paths, and
allowed recovery actions but removes its display `message`. The Web chooses
copy from `error_code`/`conflict_type`; it does not infer semantics from a path
or error string.

Already-open streams use `{error_code, params, retryable}`. Partial-result and
operation-status DTOs use `error_code` and optional `error_params`; they do not
put a raw cause in `error`, `error_message`, `partial_reason`, or `message` when
that field is intended as ordinary failure copy. A DTO may retain a separately
named sanitized diagnostic field only under the operator-diagnostic contract.

## Fixed Server contract

- Define one leaf error-code/descriptor registry below `server/internal/api`
  so the standard responder and typed DTO extensions can share it without
  package cycles.
- A descriptor owns the stable code, normal HTTP status, and allowed parameter
  keys. Handlers select a descriptor and pass the internal `cause`; they never
  pass a user-facing sentence.
- The responder writes only the public envelope. It attaches the internal
  cause, stable code, status, method, and normalized route to the existing
  structured request-error logging path. The cause never returns to the
  client.
- Remove the string-accepting `GinError`, `GinBadRequest`,
  `GinUnauthorized`, `GinForbidden`, `GinNotFound`, `GinInternalError`, and
  `HandleError` interface once their callers are migrated. The replacement API
  must make a literal display sentence impossible to supply accidentally.
- Handler classification uses `errors.Is`/`errors.As` or an existing typed
  domain result. A touched path may not choose a public error code by matching
  `err.Error()` text. This plan does not require a domain-wide error refactor
  where the handler already has enough stable context to choose a generic code.
- Direct JSON conflict writers, middleware failures, and stream error writers
  participate in the same registry. The SPA's non-API not-found response is
  outside this contract; an unmatched `/api/v1` route is not.
- Swag annotations describe the real response type and content type for every
  error status. Binary success routes must still document JSON failures.

## Fixed Web contract

- `web/src/lib/http-commons` owns a React-free error normalizer, runtime guard,
  code accessor, and localization function. Feature code does not recreate
  `{message?: string; error?: string}` casts or parse a response body itself.
- The normalizer preserves `error_code`, safe parameters, and transport status
  where available. It distinguishes API failures from browser/network aborts
  and local `Error` instances without converting everything into an opaque
  string.
- The localization catalog is explicit and exhaustive for the generated known
  code union. Every mapping calls `t()` with a literal key and English default
  so `i18next-cli extract` remains the translation-key authority. Dynamic
  `t("apiErrors." + code)` lookup is forbidden.
- API error keys live under one `apiErrors` subtree. Feature-local operation
  fallbacks remain with their owning feature; registered product terms use the
  exact canonical labels in both places.
- The localization function receives an already localized operation fallback.
  A known actionable code may override it. Generic, unknown, missing, legacy,
  malformed, and version-skewed codes use the fallback and never display
  `message`, `error`, `JSON.stringify(error)`, or a prettified wire code.
- Business logic branches on `error_code`, not translated text. Authentication
  refresh continues to branch on HTTP 401 before error localization.
- Low-level `fetch`, XHR, binary download, and SSE adapters return or throw the
  shared structured failure. They do not manufacture an English `Error.message`
  from response text. UI flows localize at the presentation boundary so a
  current language change is respected.
- Tests and production code consume generated OpenAPI types. A runtime guard is
  still required for legacy or independently upgraded Servers; it is not a cast
  around a stale schema.

## Execution phases

### Phase 0 — Lock the failures and inventory

- Add Server regression tests that require a hidden raw cause and stable
  semantic code at the standard responder, authentication, initialization
  middleware, and one storage conflict boundary; confirm that they fail under
  the current implementation before applying the fix.
- Add Web tests requiring the same wire response to produce correct English
  and Chinese copy and requiring local parsers to ignore Server English/raw
  error text; confirm that they fail under the current implementation.
- Inventory every standard responder call, direct non-2xx JSON writer,
  documented `api.ErrorResponse`, manual Web parser, binary/manual fetch path,
  SSE error event, and user-visible operation-status error field.
- Classify each observed English message into a generic code, an actionable
  domain code, a typed recovery response, or operator-only diagnostic detail.
  Freeze the initial complete code catalog before changing handlers.
- Record anti-enumeration equivalence classes for login, registration,
  passkey, MFA, refresh, and password-reset/change responses before assigning
  codes.

Exit: the inventory has one owner and migration class for every failure
surface, and the security/localization regressions fail deterministically on
the old implementation.

### Phase 1 — Establish the typed contract and shared client

- Add the Server descriptor registry and standard language-neutral responder,
  including status/parameter validation and structured cause logging.
- Change `api.ErrorResponse` to the frozen wire shape and add generated-schema
  contract assertions for required numeric `code`, typed `error_code`, optional
  parameters, and absence of `message`/`error`.
- Implement the Web normalizer and explicit `apiErrors` localization catalog.
- Add unit tests for English, Simplified Chinese, interpolation, generic-code
  fallback, unknown future codes, legacy bodies, malformed bodies, network
  failures, aborts, and runtime language changes.
- Run extract-then-fill for the complete initial catalog and require 100%
  Simplified Chinese coverage.
- During this phase only, adapt legacy Server call sites to safe generic
  descriptors so the raw fields can be removed atomically before semantic
  migration. No compatibility adapter may continue returning the old strings.

Exit: every standard response is safe and typed, every known code is
translatable, and an old or newer peer degrades to localized operation copy.

### Phase 2 — Migrate security and bootstrap vertically

- Migrate request-origin, CSRF, authentication/optional authentication,
  authorization, rate-limit, setup, application-initialization, user account,
  passkey, MFA, session, and required-password-change paths.
- Preserve indistinguishable authentication failures and `Retry-After`
  behavior while replacing string comparisons and display messages.
- Replace Auth and bootstrap flow parsers with the shared normalizer and remove
  translation-key strings stored as fake `Error.message` values where the
  structured code can carry the state instead.
- Add handler and Web flow coverage for invalid credentials, expired session,
  rate limiting, unavailable passkeys, invalid/expired MFA, permission denial,
  and app-not-initialized behavior in both supported languages.

Exit: all pre-login and bootstrap failures are localized without changing the
security distinctions visible to an unauthenticated caller.

### Phase 3 — Migrate storage and administration vertically

- Migrate Settings, backups/restores, users, cloud credentials/import,
  Repository and Storage Location lifecycle, native host actions, scan
  submission/status, queue summaries, and Lumen/classifier availability.
- Convert `RepositoryConflictDTO` and other typed 4xx variants to the shared
  error-code semantics while preserving their recovery facts and actions.
- Replace user-visible persisted `error_message`, scan `error`/
  `partial_reason`, backup `message`, and host-action text with stable codes and
  parameters. Retain only explicitly operator-scoped sanitized detail.
- Reuse the existing feature-owned translation of Lumen discovery and Event
  rebuild status codes; do not create a second meaning for those codes in the
  transport catalog.
- Update Settings/Manage/Repository Web flows to localize through the shared
  utility and keep Storage Location / Repository terminology exact.

Exit: administration and storage recovery remain actionable in English and
Chinese after restart, and no displayed copy depends on a persisted English
sentence.

### Phase 4 — Migrate media and collection requests vertically

- Migrate Assets, search, visual search, export/download, albums, people,
  duplicates, Events, locations, species, sharing, and Agent request handlers.
- Replace behavior that classifies visual-search or other failures by scanning
  English `reason`/`error`/`message` text with stable code branching.
- Route manual fetch and binary download failures through the shared structured
  parser. Keep successful media bodies and public-share authorization behavior
  unchanged.
- Replace repeated feature casts and `Error.message`/`String(error)`/
  `JSON.stringify(error)` display fallbacks with localized feature fallbacks
  plus code-specific overrides.
- Add representative component/flow tests only where code-specific recovery
  changes the UI; pure request-to-copy mapping remains a unit-test concern.

Exit: every ordinary JSON and binary-adjacent media/collection request failure
uses the common contract and no feature contains a private API-error parser.

### Phase 5 — Converge streams and partial operations

- Change Agent SSE failures to `{error_code, params, retryable}` and remove the
  public English `message`; keep provider/internal errors in Server logs.
- Change upload-job SSE errors, batch-upload item failures, and materialization
  failures to stable codes. Preserve per-file identity and progress as data,
  but never use a queue/library error as display copy.
- Apply the same rule to remaining asynchronous cloud, restore, scan, Event,
  and host-action status DTOs discovered in Phase 0.
- Update low-level clients to preserve structured errors until the owning flow
  localizes them. Verify SSE-to-poll fallback and cancellation/abort behavior
  do not turn intentional cancellation into an error toast.
- Add browser-capability tests only for behavior that truly requires SSE,
  fetch, or XHR. Keep translation and normalization assertions in React-free
  unit tests.

Exit: a failure that occurs before, during, or after a request reaches the same
localized recovery copy without exposing raw transport or worker text.

### Phase 6 — Enforce, document, and close

- Extend `server/tools/architecturecheck` to reject raw `err.Error()` in API/SSE
  payloads, string-authored standard error copy, unregistered codes, forbidden
  parameters, and undocumented direct non-2xx API JSON writers. Keep narrow
  allowlists for explicit operator diagnostics.
- Add a Web gate that rejects legacy `{message?: string; error?: string}` API
  parsing and proves every generated known code has a literal extracted
  translation mapping with English and Simplified Chinese values.
- Update `BACKEND.md` with the error registry, logging, and error-surface
  taxonomy; update `FRONTEND.md` with the normalization/localization boundary.
  Update affected feature `doc.ts` sources where error recovery is part of the
  documented flow, then regenerate their sibling `doc.md` files.
- Regenerate OpenAPI, TypeScript, ReDoc, feature docs, and i18n artifacts only
  through their canonical commands.
- Complete this plan per [README.md](README.md): extract durable alternatives
  and decisions into `.agents/decisions/`, move surviving debt to the debt
  tracker, and delete this file.

Exit: the gates fail on a newly introduced raw error or untranslated code, the
owning architecture docs describe the final boundary, and all validation below
passes.

## Validation boundaries

### Contract and security invariants

- The JSON body's numeric `code` equals the HTTP status and `error_code` is
  present for every non-2xx `/api/v1` JSON response.
- Neither the standard envelope nor a user-facing stream/operation status
  contains a raw cause, English display sentence, secret, filesystem path, SQL
  text, provider body, or stack/library error.
- Each public code has one registered meaning, normal status, parameter
  allowlist, and extracted English/Simplified Chinese mapping.
- Authentication-equivalent failures remain equivalent after migration.
- Business logic produces the same status and recovery facts as before; only
  the error representation and presentation boundary change.

### User-observable scenarios

- Invalid credentials, expired session, rate limiting, app-not-initialized,
  permission denial, Repository unavailable/conflict, storage risk
  confirmation, service unavailable, invalid media request, and unknown
  internal failure all render localized operation-appropriate copy.
- Changing the UI language and repeating the same failure changes the copy
  without restarting the Server or changing the wire response.
- Unknown future codes, legacy bodies, empty/non-JSON responses, offline
  requests, and aborted requests use intentional localized behavior and never
  stringify a payload.
- Agent stream failure, upload item failure, upload materialization failure,
  scan/restore/host-action failure, and their retry paths display stable
  localized copy while diagnostic detail remains available only where its
  operator surface permits it.
- English and Chinese copy uses the canonical product terms, including
  Storage Location / 存储位置, Repository / 资源库, and the registered Lumen
  capability labels.

### Completion evidence

Run the narrow phase-specific tests while editing. Before completing the plan,
the final combined diff requires:

```text
task architecture:check
task server:test
task dto
task web:i18n:extract
cd web && vp exec i18next-cli status
task web:test
cd web && vp run test:bundle
task verify:generated
task ci:site
```

`i18next-cli status` must report 100% Simplified Chinese coverage. Generated
files are reviewed after `task dto`/documentation generation and are never
hand-edited. A phase touching a browser E2E-only recovery contract selects its
existing E2E slice through `lumilio-select-checks`; this plan does not create a
blanket new E2E suite.

## Progress

- [x] Initial cross-layer survey and target contracts frozen.
- [ ] Phase 0 — Lock the failures and inventory.
- [ ] Phase 1 — Establish the typed contract and shared client.
- [ ] Phase 2 — Migrate security and bootstrap vertically.
- [ ] Phase 3 — Migrate storage and administration vertically.
- [ ] Phase 4 — Migrate media and collection requests vertically.
- [ ] Phase 5 — Converge streams and partial operations.
- [ ] Phase 6 — Enforce, document, and close.

## Decisions (frozen)

- 2026-08-17: Localization belongs to the Web presentation boundary. Server
  locale negotiation was rejected because it would duplicate translation
  ownership, make persisted/streamed failures language-dependent, and still
  leave browser/network failures untranslated.
- 2026-08-17: Retain numeric `code` for compatibility and add semantic
  `error_code`; repurposing `code` from number to string was rejected as an
  unnecessary wire break.
- 2026-08-17: Remove public `message` and `error` instead of retaining an
  English compatibility fallback. Old/new peers already have operation
  fallbacks, while retaining those fields would preserve both untranslated
  copy and the raw-error leak.
- 2026-08-17: Use a closed Server registry plus an exhaustive generated-type
  Web mapping. Dynamic translation-key construction was rejected because the
  extractor cannot prove coverage and unknown codes would leak implementation
  vocabulary into the UI.
- 2026-08-17: Keep typed recovery facts in specialized DTOs rather than placing
  arbitrary objects inside the common envelope. This preserves useful OpenAPI
  types and prevents `params` from becoming an unbounded detail channel.
- 2026-08-17: Ordinary users see stable localized copy; raw/sanitized technical
  detail is limited to explicitly operator-owned diagnostic surfaces and
  structured logs.

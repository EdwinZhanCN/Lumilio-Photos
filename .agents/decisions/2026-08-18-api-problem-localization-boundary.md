# Decision: Localize API Problems at the Web presentation boundary

Status: implemented

## Problem

The Server returned a custom `{code,message,error}` envelope whose English
message and raw Go cause were repeatedly recovered through feature-local casts
and rendered by the Web. The response was therefore both an accidental copy
catalog and a diagnostics leak. Request errors, SSE events, upload items, and
durable operation receipts also used different string fields, so a runtime
language change could not translate one failure consistently.

## Decision

Every non-2xx JSON response below `/api/v1` is a language-neutral RFC 9457
Problem with `application/problem+json`. It contains required `type`, `status`,
and opaque `instance` members plus only fields declared by an exact registered
subtype. It contains no `title`, `detail`, numeric code, message, raw cause, or
untyped extension map. `about:blank` covers status-only failures; distinct
Lumilio URIs exist only for different explanation, recovery, or machine action.

`server/internal/api/problem` is the closed descriptor registry. Handlers and
pre-handler middleware select a typed failure and pass the private diagnostic
cause to `api.WriteProblem`, the only HTTP emitter. The writer generates the
occurrence URI and adds instance, type, and cause to the single structured
normalized-route request log; the cause never enters the response. Gin handlers
keep their ordinary signature and share this closed writer with middleware.

Failures after request acceptance use the generated transport-neutral Problem
Reference union. It carries the same `type` and opaque `instance`, optional
retryability, and bounded subtype fields without HTTP `status`. Durable
operation instances are deterministically opaque from operation identity, so
polling and restart retain one support reference without exposing the identity.

The Web consumes only generated Problem unions through
`lib/http-commons/problem.ts`. Low-level fetch, XHR, binary, and SSE adapters
preserve structured failure states. Rendering supplies current `t` plus an
already-localized operation fallback; an exhaustive literal type switch may
override it for known actionable types. Unknown, malformed, network, abort, and
future-version failures never display wire content. Business logic branches on
status or exact type, never translated text.

The registry generates exact OpenAPI discriminator unions and public type-URI
pages. Architecture checks require every registered URI to appear in OpenAPI,
the Web literal mapping, English and Simplified Chinese catalogs, and public
documentation, while rejecting legacy responders, direct non-success `c.JSON`,
raw public errors, dynamic translation keys, and private Problem parsers.

## Alternatives considered

**Translate in the Server from `Accept-Language`** — rejected because it would
split copy ownership, make persisted/streamed failures language-dependent, and
still leave browser/network failures outside the model.

**Keep or dual-write `{code,message,error}`** — rejected because no released
client requires compatibility and a second semantic identifier would create
permanent drift and preserve the diagnostics leak.

**Return RFC `title` or `detail` as English fallback** — rejected because any
display sentence on the wire becomes an attractive accidental UI dependency.
Stable type pages document the API; operation-local Web copy handles display.

**Use dynamic translation keys derived from the type URI** — rejected because
extractors cannot prove coverage and URI prettification is not product copy.
The literal exhaustive switch makes a new generated type a compile-time and
architecture-gate obligation.

**Use separate error vocabularies for HTTP, streams, and durable jobs** —
rejected because the same semantic failure would translate and recover
differently by timing. Problem References reuse the registered type vocabulary
without pretending an asynchronous receipt is an HTTP response.

**Convert all 216 Gin handlers to a second error-returning adapter signature**
— rejected after implementation review. Gin middleware and handlers already
share one request context, and the closed typed writer makes display strings and
arbitrary bodies unrepresentable. A second routing adapter would require broad
signature/interface indirection without adding a stronger safety invariant;
the deterministic gate instead enforces the single writer boundary directly.

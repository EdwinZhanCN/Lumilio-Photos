---
name: lumilio-api-contract-change
description: Use when changing any server DTO, handler annotation, endpoint,
  or SQL schema/query in Lumilio Photos, or when a frontend response type
  looks wrong or forces a cast — regenerates OpenAPI, frontend types, and
  docs together so the contract never drifts.
---

# Change An API Contract

OpenAPI is the source of truth for HTTP contracts. The generated artifacts —
`server/docs` (OpenAPI), `web/src/lib/http-commons/schema.d.ts` (frontend
types), and `site/docs/public/redoc-static.html` — are never hand-edited.
`task verify:generated` regenerates them in CI and fails on drift. Contracts:
[BACKEND.md](../../../docs/BACKEND.md),
[FRONTEND.md](../../../docs/FRONTEND.md).

## Procedure

1. Change the backend first: DTO in `server/internal/api/dto`, handler
   annotation (`@Success ... {data=dto.X}`), and behavior together. Do not
   add frontend work against a stale type.
2. SQL schema or queries changed? `task server:sqlc` (generated repo layer
   lives under `server/internal/db/repo`). Historical migrations in
   `server/migrations` are immutable — checksums are recorded at apply time.
3. From the repository root: `task dto`. It runs `server:openapi` (swag v2),
   `web:openapi-types`, and `site:openapi-docs`.
4. Inspect `git diff web/src/lib/http-commons/schema.d.ts` and confirm the
   endpoint exposes the expected fields. `data?: Record<string, never>` or
   `data?: unknown` on a payload-returning endpoint means the backend
   annotation or DTO is broken — fix it and regenerate before any frontend
   work.
5. Frontend consumes through `$api` from `src/lib/http-commons/queryClient.ts`
   (`useQuery` / `useInfiniteQuery` / `useMutation`) with generated types.
   Do not create ad-hoc request/response types when an endpoint exists in
   OpenAPI. Do not post-edit `schema.d.ts`.
6. Verify per [lumilio-select-checks](../lumilio-select-checks/SKILL.md):
   `task server:test` and `task web:test` at minimum for a contract change.

Config profile or schema comments? `task config:examples`, never hand-edit
`server/config/examples/` or `server/config/schema/lumilio-server.schema.json`.

## The cast triage

An `as` cast on an API response is a red flag, never a fix. When a response
type looks wrong:

1. Check the handler's `@Success` annotation — does the referenced DTO declare
   the fields?
2. If the DTO is correct but `schema.d.ts` is stale or untyped, the generated
   contract is broken: fix backend DTO/annotation/codegen and rerun `task dto`.
3. Only then read the now-typed field.

Never add frontend compatibility shims, endpoint-local casts, or hand-written
response types around stale DTO output. Runtime guards are allowed only as
defensive checks after the generated contract is correct; they are not a
substitute for fixing OpenAPI.

A stale `task dto` once surfaced `camera_models` as untyped; a frontend cast
guessed `cameras` and silently broke the camera mention feature.

## Known quirk

Swag v2 emits an extra empty-object `oneOf` branch for body parameters in
OpenAPI 3.1. `web/scripts/generate-openapi-types.ts` strips that branch only
for required JSON request bodies, in memory, before type generation. Optional
empty payloads remain optional. Backend annotations and DTO validation tags
still define the contract — do not compensate in handlers or the frontend,
and do not post-edit `schema.d.ts` to hide it.

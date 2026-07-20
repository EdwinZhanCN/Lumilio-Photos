# ADR-003 Upload Hash Implementation

## Goal

Implement the accepted layered upload hashing contract without legacy-data
migration work. The project has no production deployment, so the baseline
schema may change directly.

## Scope

1. Replace `assets.hash` with authoritative `content_hash` plus optional
   `quick_fingerprint` and `quick_fingerprint_version`.
2. Make every materialization path calculate a full server-side BLAKE3 content
   hash before creating an asset.
3. Treat large-file quick fingerprints as precheck candidates only. A quick
   match must never skip transport or become an asset identity.
4. Make final duplicate detection use `repository_id + content_hash + file_size`
   and serialize concurrent upload materialization without preventing the
   scanner from representing real duplicate files.
5. Add focused contract tests and run the generated-code and project gates.

## Completed Follow-up Slice

- Upload sessions are explicitly created and persisted under repository
  staging; valid completed chunks are restored after interruption or restart.
- The browser reuses the stable session for a selected file, queries completed
  chunk indexes, skips them, and retries transient chunk failures.
- Ingest lifecycle streams over SSE with heartbeat and ownership filtering;
  the client falls back to `/batch/jobs` polling if the stream is unavailable.

## Validation

- `cd server && sqlc generate`
- focused Go and web tests while iterating
- `make server-test`
- `make web-test`
- `PRODUCTION_SMOKE_PORT=4175 make web-browser-test`

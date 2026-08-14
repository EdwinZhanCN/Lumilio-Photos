---
name: lumilio-lumen-fixtures
description: Use when changing Lumen inference, fakelumen, semantic/face/OCR/
  BioCLIP E2E behavior, or recorded fixtures in Lumilio Photos — record real
  Hub responses once, replay them keyless and GPU-less in CI, and review
  every fixture diff.
---

# Record And Replay Lumen Inference

`server/tools/fakelumen` is the external inference boundary used by the
isolated E2E stack (`lumen-fixture` in `web/e2e/compose.yml`). Product code
still exercises discovery, gRPC streaming, image preprocessing, queues,
SQLite vector storage, retrieval, and best-frame selection; only model
inference is faked.

Default mode is **replay**. Lookups are `(task, sha256(payload))`. Without a
recorded capability set the fixture advertises the builtin deterministic
SigLIP capability and answers every semantic request with one constant
768-dimensional vector — that is the legacy behavior, and it is still what
CI runs until real fixtures are committed. Misses fall back to that builtin
response and increment `fixture_misses` on `GET :16658/metrics`. `-strict`
turns a miss into an error; do not enable it in CI until the recorded set
covers the slices that run.

Recording is explicit and never implicit in CI. Every fixture diff is
reviewed.

## Record

A real Lumen Hub must be reachable from Docker at
`host.docker.internal:50051` (override with
`LUMILIO_LUMEN_RECORD_UPSTREAM`). Then, from the repository root:

```sh
task lumen:record          # compose.yml + compose.record.yml, rebuilds lumen-fixture
# seed and run the slices whose fixtures you want to refresh, e.g.:
task web:test:browser
task web:test:video-semantic
task web:e2e:down
```

`compose.record.yml` mounts `server/tools/fakelumen/fixtures` into the
container and runs fakelumen with `-record -upstream … -fixtures /record-out`.
The recording container runs as root so it can write the bind mount; `chown`
the resulting files if your umask leaves them root-owned.

What to review in the diff:

- `fixtures/manifest.json` — `recordedFrom`, `recordedAt`, and the protojson
  capability set. Replay will advertise exactly this set.
- `fixtures/records/<task>/<sha256>.json` — payload hash, mime, optional
  inline text (small `text/*` only), and `resultJson`. Binary image/tensor
  payloads stay hash-only on purpose; do not paste them in.

Re-recording the same `(task, payload)` pair overwrites in place. A
preprocessing change (libvips, SDK tensor path, seed media) produces new
hashes; delete stale records in the same PR rather than accumulating
orphans.

## Replay locally

```sh
task web:e2e:up            # default: embedded fixtures, builtin fallback on miss
```

Inspect `http://127.0.0.1:16658/metrics` after a slice. The legacy keys
`semantic_image` / `semantic_text` stay so existing Playwright assertions
keep working; `fixture_hits` / `fixture_misses` / `recorded` are the
record/replay counters.

## Unit tests vs E2E

Queue and service tests stub `LumenService` in-process — they do not talk to
fakelumen. Do not replace those stubs with recorded gRPC fixtures; the
record/replay boundary is the external Hub contract, not the Go interface.
The opt-in tensor conformance test
(`LUMILIO_LUMEN_CONFORMANCE_ADDR`) still needs a real Hub and is not CI.

## Verify

```sh
cd server && go test ./tools/fakelumen/
task compose:test          # includes compose.record.yml
```

Then run the E2E slice whose fixtures changed. A fixture-only change still
needs `web:test:browser` or `web:test:video-semantic` according to which
records moved.

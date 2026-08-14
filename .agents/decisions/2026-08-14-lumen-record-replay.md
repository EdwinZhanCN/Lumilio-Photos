# Decision: Record/replay Lumen inference at fakelumen

Status: implemented

## Problem

The isolated E2E stack already replaced the Hub with `fakelumen`, but the
fake returned one constant 768-dimensional embedding for every semantic
request and unimplemented every other task. That is enough to prove queues,
gRPC, and retrieval wiring; it cannot catch preprocessing drift, ranking
mistakes, or OCR/face/BioCLIP contracts. Hitting a real Hub in CI needs a
GPU, a key, and non-deterministic weights.

## Decision

`server/tools/fakelumen` is a record/replay boundary on the public Lumen
gRPC contract.

- **Replay** (default, what CI runs): look up `(task, sha256(payload))` in
  `server/tools/fakelumen/fixtures`. A miss falls back to the historical
  deterministic builtin response and is counted on `/metrics`. `-strict`
  is reserved until a recorded set covers the slices that run.
- **Record** (`task lumen:record`, overlay `web/e2e/compose.record.yml`):
  proxy to a real Hub, persist each exchange, and store the upstream
  capability set in `manifest.json` so replay advertises the same services.
  Recording is explicit and never implicit in CI. Every fixture diff is
  reviewed.

Queue and service unit tests continue to stub `LumenService` in-process.
The tensor conformance test stays opt-in against a real Hub. Procedure:
[lumilio-lumen-fixtures](../skills/lumilio-lumen-fixtures/SKILL.md).

## Alternatives considered

**Keep the constant-vector fake forever** — rejected for ML-contract
coverage. It remains the no-fixture fallback so this change does not break
existing slices.

**Record at the Go `LumenService` interface** — rejected: that would freeze
Photos' wrapper types and miss gRPC streaming, capability advertisement, and
the tensor fast path the product actually speaks.

**Call a real Hub in CI** — rejected: GPU, credentials, and non-determinism.
The interesting CI property is "the product still consumes this Hub
contract", which replay of reviewed bytes gives.

**Implicit recording when a miss happens in CI** — rejected: unreviewed
model output would land as a golden file. Recording is a deliberate local
(or overlay) run.

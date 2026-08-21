# Decision: Read stored OCR through bounded consumer projections

Status: implemented

## Problem

OCR Text Recognition results were persisted in SQLite and searchable through
Bleve, but users could not read them in the photo Viewer and the Agent could
only discover matching assets. The existing asset-detail relation also
silently discarded populated OCR aggregates because SQLite JSON emitted
Unix-microsecond timestamps while the public DTO decoder expected
`time.Time`. The relation sorted text by confidence rather than the provider's
returned order, which is a poor reading order and differs from the insertion
order already preserved by persistence.

The two consumers have different trust and payload boundaries. The Viewer
needs one active photo through the typed HTTP cache; the Agent needs a small,
owner-authorized, sanitized structured observation without gaining asset IDs,
paths, model internals, geometry, or unbounded user-controlled text.

## Decision

SQLite `ocr_results` and `ocr_text_items` remain the content authority. Bleve
continues to decide only OCR search membership and relevance. Readable region
order is the provider insertion order represented by `ocr_text_items.id ASC`.
The asset-detail converter decodes the internal Unix-microsecond aggregate
timestamps and maps them to the unchanged public time-based DTO.

The Viewer reuses `GET /api/v1/assets/{id}` with only `include_ocr=true`. The
query is owned by `FullScreenBasicInfo`, enabled only for the active PHOTO, and
rekeys when the active physical component changes. `PhotoInfoView` receives
plain query state and renders a colocated section with distinct loading,
missing-result, stored zero-item, populated, and retry states. Copy joins
ordered non-empty regions with newlines. Stored OCR remains readable when the
current OCR capability or setting is unavailable because opening the panel
never starts inference.

The Agent uses a separate `AgentReadOCRDocuments` sqlc projection through its
owner-bound `AuthorizedLibrary`. `read_ocr` accepts exactly one ref handle,
rejects empty refs and refs larger than two assets, restores snapshot order,
and returns per-document statuses. It exposes only a 1-based position,
sanitized filename, status, region count, ordered lines, and truncation flag.
Each line is capped at 500 runes and the two-document response shares a 6,000
rune OCR-text budget. The tool is available in the full/default set and
Analyze mode; other constrained modes do not see it. Side-channel messages
carry only counts and ref metadata, never recognized content.

The real Agent runtime proof keeps test behavior outside shipping Server code.
The external fake Lumen advertises a deterministic OCR capability, while
general E2E seeding keeps OCR disabled. The scenario temporarily enables OCR
through Settings, uploads through the public API, waits for the real queue and
SQLite write, disables OCR again, opens the real Viewer, contributes its
one-asset context ref, and lets the deterministic Ollama fixture call the real
`read_ocr` tool. A second authenticated owner cannot attach or read that
asset.

The final boundaries are documented in
[`BACKEND.md`](../../site/docs/internal/BACKEND.md#ml-lumen-and-llm) and the
Assets feature documentation.

## Alternatives considered

**Read display text from Bleve** — rejected because the index is rebuildable
and may be stale or absent; it is a membership/ranking structure rather than
the persisted content authority.

**Add a dedicated asset OCR endpoint immediately** — rejected because the
existing typed, opt-in detail relation already gives the one-photo Viewer the
right cache and authorization boundary. A dedicated route is justified only
if later payload or performance evidence requires it.

**Reuse the broad HTTP relation DTO or bare queries inside Agent tools** —
rejected because transport DTOs expose model and geometry details the Agent
does not need, while bare queries bypass the owner-bound library boundary.

**Fold OCR into `inspect` or introduce a generic text reader** — rejected
because `inspect` owns EXIF facts and audio transcript reading has a separate
media contract. Explicit observers keep authorization, budgets, statuses, and
model guidance understandable.

**Sort readable text by confidence or add an ordinal migration now** — rejected
because confidence is not reading order and persistence already serializes the
provider response. A new ordinal is warranted only when a real manual or
multi-column ordering requirement appears.

**Return every region or implicitly sample oversized refs** — rejected because
OCR is untrusted, user-controlled model input. Explicit cardinality and rune
budgets make cost predictable; narrowing with existing ref transformers keeps
the model's selection visible.

**Insert OCR rows directly into the E2E catalog** — rejected because it would
bypass the public settings, upload, queue, Lumen, persistence, context,
authorization, and tool boundaries the regression is meant to protect.

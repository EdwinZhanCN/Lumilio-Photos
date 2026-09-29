# Decision: One Monitor snapshot shell, four meaningful visual media

Status: implemented, 2026-09-12. This completes the Monitor visual-redesign plan.
The Processing bullet is superseded by
[the two-pattern Processing decision](2026-09-22-processing-two-patterns.md).
The source of architecture documentation is
[Monitor doc.ts](../../web/src/features/monitor/doc.ts).

## Problem

Monitor repeated statistics across cards, progress bars, and tables. Different
refresh/error layouts made the four tabs harder to read, and a failed background
read hid useful previous observations. The chosen visual direction must preserve
operator facts and actions without inventing completion history or topology.

## Decision

Use one information order: compact snapshot summary and refresh, primary visual,
selected detail, then expandable diagnostics. Query results remain TanStack Query
state; selections and disclosures remain local UI state. Failed refreshes retain
cached data with an explicit stale warning. A missing snapshot never means zero
work or ready. Storage is a manual snapshot; indexing still polls every fifteen
seconds, and Processing/capabilities every five seconds. Refresh only reads.

The compact heading carries exactly two things: when the shown snapshot was read,
and that tab's actions. The route header already names the tab, and each primary
visual already enumerates its own subjects, so the heading repeats neither. A
healthy subject states nothing; a count of subjects needing attention appears
only above zero and is suppressed while the shown snapshot is stale, because a
stale read cannot assert current state. Explanation reaches the operator through
selection and expandable diagnostics, not through standing prose beside a
repeated value.

- **ML:** five equal 10×10 coverage fields, collapsing from five to three to two
  columns by container width. A cell approximates one percent, not one file, and
  task denominators are independent. No applicable content is distinct from
  complete coverage. Selection exposes remaining work and queued jobs in one
  detail area. Missing-only/full rebuild confirmation and Repository scope
  survive; BioCLIP remains display-only here.
- **Storage:** gapless rectangles partition filesystem capacity. Used is total
  minus filesystem-writable space, not strictly physical occupancy. The Server's
  write budget is authoritative; the retained region is available minus that
  budget, bounded by actual available capacity. The configured reserve target
  stays separate in detail. Small areas use adjacent numeric legends rather than
  expanding geometry to fit labels. The tree means ownership only and is the only
  enumeration of targets, so the heading states no location or Repository tally.
  The filesystem-writable figure is labelled as such rather than "available", so
  it cannot be read as the Server's write budget. Unknown capacity, read-only/
  offline states, identity/recovery problems, mount warnings, path copying,
  support bundles, and lifecycle audit remain accessible.
- **Capabilities:** the orbit represents actual discovered Lumen Hub endpoints,
  not inferred devices inside a Hub. Stable identity ordering and four endpoints
  per page keep controls legible at phone widths. A capability filter locates
  endpoints advertising any of its constituent tasks; advertisements do not
  override the public capability's enabled/available facts or composition rules.
  Solid/filled and dashed/hollow nodes have explicit state labels. Distances
  encode no latency or performance. Transport, compatibility, discovery,
  endpoint/source/version/last-observation detail, typed errors, and Agent
  configuration remain distinct and reachable.
- **Processing:** four independently selected Catalog work types keep exact
  pending and attention counts. Retry-waiting stages are a qualifier, never a
  fifth stack or a file count. Only files use the author's Rive; the other three
  types use DOM paper/tray geometry with the same fixed eighteen-layer reference.
  Units are 32 files, one Repository, two projections, or one operation per
  layer. Rive's four quantised tiers are not a fabricated batch-completion
  percentage. Pending zero with failures is not ready. Delivery history and
  queue error-copy diagnostics remain separately labelled inside a disclosure.

The runtime/asset contract is recorded in
[the Rive decision](2026-09-10-rive-processing-tray-runtime.md).

## Alternatives rejected

- Repeated per-task cards and progress bars obscure selection and duplicate
  the same facts. The selected hundred-cell field provides the coverage view.
- Fixed minimum sizes or gaps inside capacity partitions distort real ratios.
  Geometry has no gaps; labels can move out of small regions.
- An unlimited orbit shrinks controls or overlaps nodes. Pagination preserves
  every observed endpoint, and the full diagnostic table remains available.
- Deriving capability availability from advertised tasks confuses discovery
  with the product's composed readiness. The public API remains authoritative.
- Four identical Rive instances contradict the selected one-asset/DOM design.
  Only the file tray loads the asset, with static equivalents for failure and
  reduced motion. No artwork was re-exported or modified in this continuation.
- Replacing previous data on refresh failure loses context. Keeping an explicit
  stale snapshot preserves evidence without asserting current health.
- A heading that repeats the route title and re-tallies what the primary visual
  already enumerates reads as three statements of one fact. Standing sentences
  that explain the layout, restate a healthy state, or narrate a value already
  printed beside them were deleted rather than restyled; only a measurement
  caveat no other surface owns survives, inside the diagnostics disclosure.

## Validation

- `task web:test`: 506 passed, 6 environment-dependent skips; type, lint, and
  source-boundary checks pass.
- Browser Mode checks cover 390/800/1280px, light/dark themes, actual painted
  Rive pixels, native Chromium reduced-motion emulation, capability selection
  across pages, scoped rebuild requests, and failed refresh retention.
- Capacity areas are checked using real browser pixel geometry. A deliberate
  5% height distortion produced a 0.58897 area fraction instead of 0.62 and
  failed the guard; the correct geometry was restored and the gate passed.
- `vp run test:bundle`: self-hosted `.riv` and WASM emitted in the lazy Monitor
  path; entry 184.9 KiB gzip against a 420 KiB budget.
- `task architecture:check`: passes, including canonical terminology.
- `task web:docs` regenerates the feature documentation; i18n uses
  extract-then-fill. No canonical terminology row changed.

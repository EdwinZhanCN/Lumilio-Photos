# Decision: Serve the Processing tray from a self-hosted Rive runtime

Status: implemented, 2026-09-10. Runtime wiring, tray component, and rules are
in `web/src/features/monitor/modules/rive/` and
`web/src/features/monitor/model/processingProgress.ts`.

## Problem

Monitor's Processing view needs an authored animation: a stack of work that
thins as the backlog drains. Rive is the only way to play the owner-authored
`.riv`, and the Rive web runtime brings three problems a local-first app cannot
ignore.

First, left alone the runtime fetches its WASM from a public CDN. A picture
library that manages local files must boot and stay correct with no network, and
a test suite must not depend on a third party either.

Second, the asset is a binary that has to reach the browser. Copying it into
`public/` would create a checked-in generated artifact with no freshness gate.

Third, the asset's own interface decides what the product may claim: it exposes
one driven number and four discrete timelines, and it contains no error or
completion graphics at all.

## Decision

Use `@rive-app/react-canvas` with the Canvas2D renderer, and self-host
everything:

- `RuntimeLoader.setWasmUrl(wasmUrl)` points at the WASM emitted by the bundler
  from `@rive-app/canvas/rive.wasm?url`, and `setWasmFallbackUrl(null)` disables
  the CDN outright rather than merely overriding it. A browser with no network
  egress still loads the tray, which is exactly the proof wanted.
- The `.riv` is imported with `import.meta.glob(..., { query: "?url" })`, the
  pattern this repository already uses for internal assets, so the bundler
  hashes and emits it. The source-boundary checker only reads static imports, so
  this needs no gate change.
- Only `progress` is driven, as a completion percentage where 100 is an empty
  tray. It comes from a fixed reference — eighteen layers times a per-work-type
  unit — never from the current maximum, so real progress cannot be masked by a
  re-scaled axis.
- Status, attention, and exact counts stay in the DOM. The canvas is decorative
  to assistive technology, and the surrounding text carries every fact whether
  the runtime loads, fails, or is skipped for reduced motion.

## Alternatives considered

**The WebGL2 renderer** (`@rive-app/react-webgl2`) — rejected. It was tested
against this asset with a real WebGL2 context: identical result, no functional
gain, a larger WASM than the Canvas2D build, and a browser WebGL context limit
that would bite if more than one tray ever animates at once. The asset was also
authored and visually signed off against Canvas2D. Switching later is a package
swap with an identical API, so nothing is locked in.

**Copy the WASM and the asset into `web/public/`** — rejected. It produces
unhashed duplicates of files that already exist in `node_modules`, and the
harness requires every checked-in generated artifact to carry a freshness gate
or a named exception. Bundler-emitted assets need neither.

**Drive `isProcessing` and `needsAttention`** — rejected. They exist in the view
model for compatibility with older instances and the current animation does not
consume them. Building product meaning on inputs the artwork ignores would
promise a behaviour the asset does not have.

**Recompute the tray scale from the largest backlog seen** — rejected. It would
keep the tray looking full while the real backlog drained, and the Server
exposes no batch total to divide by, so any such denominator would be invented.

**Re-export the asset to add finer tiers** — rejected by the owner. Four
discrete tiers are correct; continuous movement belongs to the surrounding
interface, not the artwork.

**Treat the shipped asset as authoritative and adapt the runtime to it** —
rejected after measurement. The copy originally committed under
`web/src/assets/rive/` is a different export that the runtime rejects outright
(`runtime.load()` returns null). The owner's `golden-inbox.riv` loads in the
identical harness on both renderers with a byte-identical WASM, so the working
file replaced it and the unusable copy was removed.

## Interface and integration reconciled, 2026-09-12

The pinned versions are `@rive-app/react-canvas@4.34.1` and
`@rive-app/canvas@2.42.0` (exact dependency specifiers). The authored interface is:

- artboard `File Processing — Golden Inbox`, 512 × 512;
- state machine `File Processing`, data binding rather than state inputs;
- driven number `progress`, 0–100, where 100 is empty;
- timelines `Full`, `Two Thirds`, `One Third`, and `Empty`;
- tier thresholds below 100/3, below 200/3, below 100, and exactly 100.

`assetContract.browser.test.ts` enumerates this real interface. The file work
tray explicitly fetches the bundled asset into a buffer, sets the current
progress before playback, and renders a static equivalent on failure or reduced
motion. Other work types use DOM stacks; they do not load additional Rive
instances. A component guard checks nontransparent painted pixels as well as
loading state, and native Chromium reduced-motion emulation proves no canvas
is created. The first cached implementation had used `src` and four instances;
this continuation brought it into agreement with the frozen design.

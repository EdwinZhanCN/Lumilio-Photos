# Studio Gaps: Border Exclusivity, State Reset, Crop, Manual Adjust, Depth Analysis

## Context

`web/src/features/studio` is a partial replica of the vendored AfterFrame editor (`3rd-party/AfterFrame/`, MIT, Copyright (c) 2026 Yi Wang). Six gaps were identified against the reference:

1. **Border modes stack** — presets and custom border both mutate the same CanvasSpec with no exclusivity. User decision: they are mutually exclusive; switching preset→custom **clears canvas to zero**. Additionally, the frosted background gains a device-info caption rendered centered in the border band.
2. **State reset is broken** — `resetAll` only resets adjustments; composition (canvas/layers) is not undoable at all; no base-snapshot concept.
3. **Crop is a stub** — `adjustments.crop` exists in types/DTO/sidecar but has zero implementation (no UI, no worker rendering).
4. **No manual adjustment** — no viewport drag for text layers; no zPosition.
5. **No depth analysis** — AfterFrame uses CoreML Depth Anything V2 Small (F16) via a macOS-only Swift CLI (518x392 grayscale, 255=nearest). We replicate it in-browser with transformers.js v3 + `onnx-community/depth-anything-v2-small` ONNX (fp16 ~49MB, WebGPU-accelerated, runtime-downloaded and cache-cached — never bundled).
6. **No attribution** — AfterFrame/MIT is not credited anywhere.

Confirmed implementation order: attribution → state system → border → crop+drag → depth.

Verified environment facts: `vite.config.ts` already has `worker.format: "es"` and COOP/COEP `credentialless` headers (the two usual transformers.js blockers); bundle budget is entry-chunk-only (420 KiB gzip) so a lazy depth worker won't trip it; sidecar is a fully typed Go DTO (`server/internal/api/dto/asset_dto.go`) so new persisted fields require DTO change + `make dto`; crop DTO (`StudioEditCropDTO`) already exists.

---

## Phase 0 — Attribution

- `web/src/features/studio/doc.ts`: add Acknowledgements section — frame template system derived from AfterFrame (`3rd-party/AfterFrame/`), MIT License, Copyright (c) 2026 Yi Wang. Regenerate `doc.md` via the docts toolchain (never hand-edit).
- Root `README.md`: add a Third-party section crediting AfterFrame (MIT) with path `3rd-party/AfterFrame/`.

---

## Phase 1 — Unified State Reset System

Foundation for all later phases: border mode transitions, crop commit, text-drag commit, and zPosition all become ordinary state mutations on one timeline.

### New: `model/editorState.ts` (React-free, unit-tested)

```ts
export type FrameMode = "preset" | "custom" | null;

export type EditorState = {
  adjustments: StudioEditAdjustments;  // photometric + geometry (crop/rotation/flips)
  canvas: CanvasSpec | null;
  layers: Layer[];
  frameMode: FrameMode;
  activeTemplateId: string | null;
};

export const DEFAULT_EDITOR_STATE: EditorState;
export function cloneState(s: EditorState): EditorState;          // structuredClone
export function stateEquals(a: EditorState, b: EditorState): boolean;  // field-wise, no JSON.stringify (hot path)
export function deriveFrameMode(s): FrameMode;  // activeTemplateId→"preset"; canvas||layers→"custom"; else null
```

Invariant: stored `frameMode` must always equal `deriveFrameMode(...)` — only transition helpers (Phase 2) write it; unit test asserts after every transition.

### New: `flows/editor/useEditorHistory.ts`

Modeled on AfterFrame `state/useEditorHistory.js`: single timeline of full EditorState snapshots, refs + one `useState`, owned by StudioEditor. Timeline reducer logic extracted to pure functions for testability.

```ts
export type EditorHistory = {
  state: EditorState;                    // current working state
  canUndo: boolean; canRedo: boolean;
  apply(patch: Partial<EditorState>): void;   // live update, NO history entry (drags/scrubs)
  commit(next?: EditorState): void;           // push snapshot, truncate redo tail, no-op if stateEquals(head)
  commitCoalesced(signature: string): void;   // 350ms debounce window; same signature collapses to ONE entry
  flushCoalesced(): void;                     // call on pointerdown so slider+drag are separate entries
  undo(): void; redo(): void;
  resetAll(): void;       // commit(clone(baseSnapshot)) — undoable return to load-time state
  resetDevelop(): void;   // adjustments → base adjustments (keep canvas/layers)
  resetBorder(): void;    // canvas=null, template layers removed, frameMode=null
  loadBase(s: EditorState): void;  // on asset/sidecar load: base = s, timeline = [s]
};
```

Internals: `timeline[0]` = base; MAX_HISTORY = 50 (drop oldest-after-base when over cap); `commitCoalesced` debounces per signature, flushes previous on signature change.

### Refactor existing files

| File | Change |
|------|--------|
| `flows/editor/StudioEditor.tsx` | Replace `adjustments` useState + `history[]` + `pushHistory`/`undo`/`resetAll` with `useEditorHistory`. `currentComposition = useMemo({canvas: state.canvas, layers: state.layers})`. Dirty signature = `JSON.stringify({adjustments, canvas, layers})` (excludes frameMode/activeTemplateId — derivable/unsaved). Render effect deps → `[asset, state, ...]`. On asset load: build EditorState from sidecar → `loadBase()`. |
| `flows/editor/useComposition.ts` | Drop ownership of canvas/layers/activeTemplateId (moved to history). Keep: templatePreviews, exifAvailable, logo rasterization. `applyTemplateById` → callback that expands template and calls injected `onApplyTemplate(expanded)`; layer-merge logic moves to StudioEditor commit handlers. |
| `flows/editor/TopBar.tsx` | Wire undo/redo/canUndo/canRedo/resetAll. **Add Redo button** (currently only Undo). |
| `flows/editor/EditorPanel.tsx` + `develop/DevelopSections.tsx` | Develop header Reset → `resetDevelop()`. |

Risk mitigation: render-effect + dirty-signature machinery stays shape-identical; do this as one isolated change before adding any new fields.

---

## Phase 2 — Border Mutual Exclusivity + Frosted Device Info

### 2.1 Mode transitions (the only three writers of frameMode, in StudioEditor)

- **Apply template → preset:** commit `{canvas: expanded.canvas, layers: [...expanded.layers, ...nonTemplateLayers], activeTemplateId: id, frameMode: "preset"}`. FramePanel: when `frameMode === "preset"`, CanvasControls **disabled** + explicit **"Customize border"** button shown.
- **Customize → custom (canvas cleared to zero):** button commits `{canvas: DEFAULT_CANVAS, layers: layers.filter(l => !l.fromTemplate), activeTemplateId: null, frameMode: "custom"}`.
- **Clear → null:** commit `{canvas: null, layers: [], activeTemplateId: null, frameMode: null}`.

Explicit button (not "any touch converts") because CanvasControls computes each `next` from the full preset canvas, making "which field changed" ambiguous.

Files: `flows/editor/frame/FramePanel.tsx` (disable + button), `flows/editor/frame/CanvasControls.tsx` (accept `disabled` prop), `flows/editor/StudioEditor.tsx` (handlers), `model/editorState.ts`.

### 2.2 Frosted deviceInfo

**Types:**
- `model/canvasSpec.ts`: frosted variant gains `deviceInfo?: boolean`; update `DEFAULT_FROSTED_BACKGROUND` + `normalizeCanvasBackground`.
- Go DTO: `StudioCanvasBackgroundDTO` gains `DeviceInfo bool json:"deviceInfo,omitempty"`. Run **`make dto`**.

**Rendering (worker-side, rotates/frames with photo):**
- `modules/rendering/renderCanvas.ts`: new `drawDeviceInfo(ctx, spec, exif, geometry)` step — draw order becomes `bg → photo → scrim → vignette → deviceInfo → outer-clip`. Renders camera model + EXIF summary line centered in the border band; reuses group-centering from `modules/frame/expandTemplate.ts` and formatting from `modules/frame/frameExif.ts`. Only when `hasSufficientExif(exif)`.
- `modules/rendering/composeStudioImage.ts`: add `exif: FrameExif | null` to composition input, thread to `renderCanvasSpec`.
- `modules/rendering/studioEdit.worker.ts`: add `exif` to RENDER_PREVIEW/EXPORT_IMAGE payload; deviceInfo text triggers font loading.
- `StudioEditor.tsx`: pass existing `frameExif` state into composition sent to worker (keep exif OUT of dirty signature — not persisted).
- `CanvasControls.tsx` (FrostedControls): `deviceInfo` toggle, shown when frosted kind active + `exifAvailable`.

---

## Phase 3 — Crop Geometry + Text Drag / zPosition

### 3.1 Crop math — new `model/cropMath.ts` (pure, exhaustively unit-tested)

Ported from AfterFrame `cropMath.js`:

```ts
export const MIN_CROP_SIZE = 48;  // px, screen space
export const ASPECT_PRESETS: { id: string; label: string; ratio: number | null }[];
  // free, original, 1:1, 3:2, 2:3, 4:3, 3:4, 5:4, 4:5, 16:9, 9:16, 2.35:1
export type CropRect = { x: number; y: number; width: number; height: number };  // normalized 0-1, un-rotated source space
export function createDefaultCropRect(): CropRect;
export function moveCropRect(rect, dx, dy, bounds): CropRect;
export function resizeCropRect(rect, handle, dx, dy, ratio, bounds): CropRect;  // 8 handles; free vs fixed-aspect
export function clampCropRect(rect, bounds): CropRect;
export function cropRectToSourcePx(rect, srcW, srcH): {sx, sy, sw, sh};
// + rotation/flip source<->display transforms (all 4 rotations x flips) — highest-risk math, exhaustive tests
```

Storage: normalized in existing `adjustments.crop` — **no DTO change needed** (StudioEditCropDTO exists).

### 3.2 Viewport crop interaction (`flows/editor/Viewport.tsx`)

New pattern (no pointer-capture exists in codebase today): 8 resize handles + interior drag, `setPointerCapture`, rule-of-thirds grid, dimmed outside region. While crop-editing: worker renders with `crop=null` and no composition (user sees full frame); crop rect lives in local overlay state (screen px); drags don't touch history; on pointerup → convert to normalized source-space rect → `commit()`. Memoize render inputs so overlay drags don't churn the worker.

### 3.3 Develop Geometry UI (`develop/DevelopSections.tsx`)

Add aspect preset grid (from ASPECT_PRESETS) + "Reset crop" button (commit crop→null). Selecting a preset enters viewport crop-editing mode.

### 3.4 Worker crop pipeline (`modules/rendering/studioEdit.worker.ts`)

- `getRenderSize()`: compute cropped source dims (`cropRectToSourcePx`) **before** rotation so output size reflects crop.
- `drawSourceCanvas()`: replace `ctx.drawImage(drawable, -w/2, -h/2)` with sub-rect form `ctx.drawImage(drawable, sx, sy, sw, sh, -sw/2, -sh/2, sw, sh)`; existing translate/rotate/scale unchanged.

### 3.5 Text drag + zPosition

- `model/layers.ts`: add `zPosition: number` to layer base (default 1.0 = fully in front); update `createTextLayer` + `normalizeLayer` (default 1.0 for old sidecars).
- Go DTO: `StudioLayerDTO` gains `ZPosition *float64 json:"zPosition,omitempty"`. Run **`make dto`**.
- Viewport: when a text layer is selected, pointer-drag updates x/y (screen px → composed-output fractions, visual center). Live via `apply()`, commit on pointerup. Reuses crop's pointer-capture pattern.
- `text/TextPanel.tsx`: per-layer zPosition slider (0-1).

---

## Phase 4 — Depth Analysis (transformers.js)

### New: `modules/depth/`

| File | Purpose |
|------|---------|
| `depth.worker.ts` | Dedicated Web Worker. Dynamic-imports `@huggingface/transformers` v3; `depth-estimation` pipeline, model `onnx-community/depth-anything-v2-small`, `{device: "webgpu"}` (wasm fallback), `dtype: "fp16"` (~49MB, runtime download, Cache API cached). One inference per source image (adjustments don't invalidate — field aligned at render time). Outputs normalized 8-bit grayscale, **255 = nearest** (matches AfterFrame convention). Posts ImageData + dims. |
| `useSceneDepth.ts` | Hook (modeled on AfterFrame `useSceneDepth.js`): one inference per source, caches depth field (canvas + dims), exposes `{depthField, status, compute, mapVisible, feather}`. Feather default 0.08, session-only. |
| `depthMask.ts` | Port of AfterFrame `buildDepthAlphaMask(depthCanvas, zPosition, feather)`: white where depth < z, transparent where depth > z, feather ramp `lo = max(0, z - f/2)`, `hi = min(1, z + f/2)`. Pure, unit-tested. |

### Render integration

- `studioEdit.worker.ts`: new `SET_DEPTH` message receives depth field. New pure `alignDepthField(depthField, adjustments, geometry)` mirrors drawSourceCanvas crop/rotation transform. In compose: layers with `zPosition < 1` draw to temp canvas → `destination-in` with mask → blit. Depth field maps onto photo content sub-rect only (borders stay visible).
- `composeStudioImage.ts`: accept aligned depth field + feather, perform per-layer masking.

### UI

Depth compute button, feather slider, depth-map visualization toggle, per-layer zPosition slider (from Phase 3).

### Dependency

`web/package.json`: add `@huggingface/transformers` v3. Add to `optimizeDeps.exclude` in `web/vite.config.ts`.

---

## Testing

- `model/cropMath.test.ts` — aspect presets, create/move/resize/clamp, all 8 handles, free vs fixed, **exhaustive rotation/flip transforms**.
- `model/editorState.test.ts` — stateEquals, deriveFrameMode invariant after every transition, clone independence.
- History timeline reducer (pure, extracted) — commit/undo/redo, coalescing collapse, no-op prevention, resetAll/resetDevelop/resetBorder, base-snapshot.
- `modules/depth/depthMask.test.ts` — feather ramp boundaries, z extremes.
- Browser tests: renderCanvas deviceInfo draw order, composeStudioImage depth masking.
- Pattern: follow `model/canvasSpec.test.ts` (`import {describe,expect,it} from "vite-plus/test"`).
- i18n: `t("key", "default")` for all new strings → `vp exec i18next-cli extract`.

## Risks

| Risk | Mitigation |
|------|-----------|
| transformers.js bundling (biggest external) | Already mitigated: `worker.format: "es"` + COOP/COEP present; model runtime-downloaded never bundled; budget check is entry-only. Dynamic import in separate worker + `optimizeDeps.exclude`. POC early in Phase 4. |
| State refactor blast radius (biggest internal) | Render-effect + dirty-signature shape-identical; isolated change before new fields; stateEquals no-op prevention. |
| `make dto` drift | New fields need Go DTO + `make dto` + frontend type sync; forgetting causes typed-client compile error (caught by `vp check`). |
| Crop rotation math | Exhaustive unit tests for all 4 rotations x flips. |

## Verification

1. `make web-test` (= `vp check --no-fmt --no-lint && vp lint && vp test`)
2. `make web-browser-test` — renderer integration tests
3. `vp node scripts/check-source-boundaries.mjs` — feature import rules
4. Bundle budget passes (depth worker is lazy)
5. `make dto` clean; `schema.d.ts` diff shows only the two new optional fields (deviceInfo, zPosition)
6. Manual: preset → controls disable → "Customize border" clears to DEFAULT_CANVAS; undo/redo single timeline with coalescing; crop survives rotation; deviceInfo renders only with sufficient EXIF; depth mask respects zPosition + feather
7. `make server-test` after DTO changes

## Critical Files

**Existing (modify):**
- `web/src/features/studio/flows/editor/StudioEditor.tsx` — state owner, worker interaction, save/sidecar
- `web/src/features/studio/modules/rendering/studioEdit.worker.ts` — render pipeline (crop + depth insertion)
- `web/src/features/studio/flows/editor/useComposition.ts` — composition ownership transfer
- `web/src/features/studio/model/canvasSpec.ts` — frosted deviceInfo variant
- `server/internal/api/dto/asset_dto.go` — DTO for deviceInfo + zPosition

**New (create):**
- `web/src/features/studio/model/editorState.ts` — unified state + FrameMode
- `web/src/features/studio/flows/editor/useEditorHistory.ts` — timeline hook
- `web/src/features/studio/model/cropMath.ts` — crop pure math
- `web/src/features/studio/modules/depth/` — depth.worker.ts, useSceneDepth.ts, depthMask.ts

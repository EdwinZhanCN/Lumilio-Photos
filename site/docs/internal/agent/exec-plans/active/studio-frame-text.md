# Studio Frame & Text

Status: active. Ports the AfterFrame frame/watermark and text-overlay system
into Studio, replacing the destructive Border tool with a sidecar-persisted,
non-destructive composition model.

Source of the port: `3rd-party/AfterFrame/apps/desktop/src/components/editor/`.

## Why

The current Border tool bakes pixels and forgets them: `handleApplyBorder`
exports the developed image, runs a one-shot worker tool, and shows a blob URL
that any adjustment change discards. Nothing survives a reload, and the five
modes are five parallel renderers with conflicting parameter semantics.

AfterFrame's model is better because a template is not a renderer. A preset
explodes into ordinary editable layers plus a canvas treatment, so "apply a
preset" and "add my own text" land in the same state and one renderer draws
both.

## Model

Three orthogonal concepts. The Border tool conflated all three; keeping them
separate is the point of this work.

### Canvas — the border layer

Owns how the photo is framed and what fills the space around it.

```ts
type CanvasSpec = {
  pad: { top; right; bottom; left };      // fractions of the SHORT edge
  background:
    | { kind: "solid"; color }
    | { kind: "gradient"; from; to; angle }
    | { kind: "frosted"; blur; brightness; overscan };
  outerRadius: number;                     // fraction of min(W,H)
  innerRadius: number;                     // fraction of min(photo w,h)
  scrim: { edge; from; to; height } | null;
  vignette: number;                        // 0 = off
};
```

Frosted lives here, not in the template. "Frosted without text" and "frosted
with text" are the same canvas with different layers — not two modes.

Unifies the two divergent frosted implementations. `basicBorders.renderFrosted`
and `exifBorderRenderer` disagreed on every parameter:

| | old `FROSTED` | old `FROSTED_INFO` | unified |
| --- | --- | --- | --- |
| background | 1:1, no overscan | overscan then blur | overscan then blur |
| inner radius | none | `min(fg) × 0.16` | `innerRadius` fraction |
| outer radius | yes | none | `outerRadius` fraction |
| foreground | hardcoded `0.75` | `fitContain` | `fitContain` from `pad` |
| `corner_radius` unit | pixels | percent | fractions, both radii |

### Layers — the content

Text and logo/image layers with a shared geometry contract (`x`/`y` as
fractions of the composed output, `rotation`, `opacity`, `shadow`). One
renderer, worker-safe.

### Templates — the presets

A template references a canvas treatment and declares anchored elements. At
apply time it expands into `(CanvasSpec, Layer[])` and is then fully editable.
Templates never carry rendering logic.

## Sidecar

Additive, no version bump. Absent fields on existing sidecars read as
"no composition", so v1 files stay valid.

```ts
LumilioSidecarV1 = {
  version: 1; asset_id; source; adjustments;   // unchanged
  canvas: CanvasSpec | null;                   // new
  layers: Layer[];                             // new
  updated_at;
}
```

Backend: extend `LumilioSidecarV1DTO` in `server/internal/api/dto/asset_dto.go`,
then `make dto`. Never hand-edit `web/src/lib/http-commons/schema.d.ts`.

## Worker

Rendering moves fully into the worker on `OffscreenCanvas`. Two deliberate
improvements over the source:

- **Fonts load in the worker** via `FontFace` + `self.fonts.add()`, so
  `measureText` runs against the real face. AfterFrame measures text in the DOM
  and renders it on canvas, and carries a long comment
  (`useFrameTool.js:20-44`) about alignment drifting proportionally to line
  width when the two disagree. Measuring where we draw removes that entire bug
  class instead of compensating for it.
- **Template thumbnails render from one shared small base** with cached
  `ImageBitmap`s rather than a full-resolution rebuild plus JPEG encode per
  template behind a 200 ms debounce.

Logo SVG rasterization stays on the main thread (`ImageBitmap` transferred in),
matching the existing worker-stays-DOM-free rule.

## Assets

### Logos

Merge into one registry with variants. The two sets overlap and are same-source
— `hasselblad` (4443 B), `fujifilm` (6530 B), `sony` (5135 B) are byte-identical
to the existing Lumilio files. Keep one copy.

- Lumilio contributes 14 wordmarks, 5 brands AfterFrame lacks: apple, olympus,
  pentax, sigma, zeiss.
- AfterFrame contributes the symbol variants templates need: `leica/symbol`,
  `hasselblad/symbol`, `sony/symbol`, `nikon/lockup`, `lumix/wordmark`.

Result: 14 brands / 19 files, manifest-driven, with per-variant `aspect`, `h`,
`color`, and `colorLocked`.

### Fonts

Bundle `@fontsource` locally — no CDN, the deployment is offline-capable.
Latin faces in full; Noto Sans SC as a common-hanzi subset (~3000 glyphs).

## Placement

```text
web/src/features/studio/
├── model/
│   ├── canvasSpec.ts        # CanvasSpec type, defaults, normalize
│   ├── layers.ts            # Layer types, factories, normalize
│   └── editTypes.ts         # sidecar, extended
├── modules/
│   ├── rendering/
│   │   ├── canvas/          # background / frosted / vignette / scrim
│   │   ├── layers/          # the one layer renderer
│   │   └── fonts/           # worker-side FontFace loading
│   └── frame/               # template catalog, logo registry, expansion
└── flows/editor/
    ├── frame/               # template picker panel
    └── text/                # text layer panel
```

`model/` stays React-free. Template names carry i18n keys, not literals;
values are filled via `vp exec i18next-cli extract` (never hand-edited).

## Sequence

1. Assets and fonts: merged logo registry, `@fontsource` deps.
2. `model/`: `CanvasSpec`, `Layer`, extended sidecar.
3. Worker rendering: canvas treatments, layer renderer, font loading.
4. Template catalog and expansion.
5. UI panels; retire the Border tool and its five-mode UI.
6. Backend DTO plus `make dto`.
7. i18n extract, tests, `doc.ts`.

## Removals

`modules/tools/border/` goes away entirely, along with `BorderToolSection`,
the border branch in `handleExport`, and every `clearBorderResult` call site.
No compatibility shims.

## Status

Engine, wiring, and panels are in. `make web-test` passes (50 files / 185 tests,
0 type errors, 0 cycles); `make dto` regenerated the contract and the server API
packages build.

Landed:

- Logo registry: 14 brands / 19 files under `brand/variant.svg` with a manifest.
- Fonts: seven `@fontsource` packages, dynamically imported so the ~3.7 MB chunk
  is not in the main bundle. Chinese uses the single-file `chinese-simplified`
  build, not the sliced one — `unicode-range` slices are only fetched when a
  matching character is in the DOM, which canvas `fillText` never triggers.
- `model/`: `CanvasSpec`, `Layer`, `StudioComposition`, and the extended sidecar.
- `modules/rendering/`: unified canvas treatment, one layer renderer, worker-side
  font loading, and the `composeStudioImage` stage.
- `modules/frame/`: registry, EXIF, 21 templates, two-pass `applyTemplate`,
  shared-base template previews.
- UI: `EditorPanel` tabs hosting `DevelopSections`, `FramePanel`, `TextPanel`,
  with `ValueSlider` extracted from `SliderRow` for reuse.
- Backend sidecar DTO plus `make dto`.

Removed: `modules/tools/` entirely, `BorderToolSection`, `DevelopPanel`,
`tool.worker.ts`, and the tool runtime on `AppWorkerClient`.

### Geometry moved into the worker

Preview used to strip rotation/flip and let `Viewport` apply them with CSS. That
cannot survive a frame: the border would rotate along with the photo. The worker
now renders geometry, and the viewport presents the result as-is.

### Open

- **The plugin/tool framework is gone.** `tool.worker.ts` and `AppWorkerClient`'s
  tool runtime existed only to run the border tool, and its contract was
  `File -> bytes` — a pure image transform. The Studio design includes
  Marketplace and Plugins views, so if that work resumes it needs a framework
  designed for it rather than this one restored.
- No characterization tests for `expandTemplate` or the renderers yet: both need
  `OffscreenCanvas`, so they belong in the Browser Mode project rather than the
  Node unit project.
- Layers are edited through panel controls only. Direct manipulation on the
  canvas (drag, resize, snap guides) is not implemented.

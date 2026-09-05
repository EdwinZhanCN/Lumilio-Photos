# Govips call audit

Audited 2026-09-05 against pinned govips v2.18.0. The source inventory covers
all seven production files importing govips; `imagesource` supplies options,
while the other six call the native wrapper directly. Tests and upstream
library implementation were read to establish the actual option semantics.

## Native reproduction

The same two user-selected JPEGs were read without modifying the media or
catalog. The local library (8.18.2) selected `VipsForeignLoadJpegBuffer`; the
installed beta.1 library (8.18.6) selected `VipsForeignLoadUhdrBuffer`. Both
libraries decoded the inputs through `thumbnail_buffer` without loader
`autorotate`. Only the UHDR loader rejected `autorotate=true`.

The fixed Go imaging code was compiled into a temporary test executable and
run with the installed app's Frameworks and VIPSHOME. Dynamic-loader output
confirmed the app's `libvips.42.dylib` was loaded. Both inputs passed small and
medium thumbnail generation and resized/full-size WebP export. The source
photos and temporary probe are not repository fixtures or CI dependencies.

This identifies an application assumption exposed by native decoder selection,
not evidence that libvips 8.18.6 cannot decode the images. The experiment does
not isolate upstream version changes from build-time codec availability.

## Call-site dispositions

| Source / operations | Disposition |
| --- | --- |
| `imaging/process.go`: `LoadThumbnailFromBuffer` in resized processing and `StreamThumbnails` | Removed JPEG/TIFF magic-byte orientation classification and loader-specific `AutoRotate` options. The thumbnail operation owns orientation; common fail-on-error import settings remain. Negative processing sizes and nonpositive thumbnail sizes are rejected. |
| `imaging/process.go`: `NewImageFromBuffer`, `AutoRotate`, `Rotate` | Full-size user export applies metadata orientation regardless of container. Explicit RAW rotation stays separate: it comes from LibRaw's container metadata. No-resize RAW encoding retains its explicit orientation ownership. |
| `imaging/process.go`: `EmbedBackground` | Existing canvas-fit check retained; negative or partially specified padding is now rejected. |
| `imaging/process.go`: `RemoveICCProfile` and WebP/JPEG/PNG/AVIF exporters | Profile-removal errors now propagate. Existing explicit encoder allowlist, metadata policy and export error propagation retained. No encoder is selected by input magic bytes. |
| `imaging/process.go`: `ToGoImage` / RGB extraction | Retained. Pinned govips converts to 8-bit sRGB (or grayscale) and forces pixel evaluation. Extraction preserves the existing HWC RGB contract and discards alpha; this audit does not change model compositing policy. |
| `imaging/ml_decode.go`: `NewImageFromBuffer`, `AutoRotate`, `ToColorSpace` | Added decoded-image orientation before model resizing; no format guessing. All error paths close the image. |
| `imaging/ml_decode.go`: `ResizeWithVScale`, `Resize`, `ExtractArea` | Positive target dimensions required. Rectangular center crop now scales the shortest edge to the larger requested dimension so the crop fits. Existing square BioCLIP and exact SigLIP contracts remain unchanged. |
| `service/face_service.go`: image decode, `AutoRotate`, `ExtractArea`, `ExportWebp` | Crop uses display-oriented coordinates, matching the face tensor path. Nonfinite, inverted, empty and wholly out-of-image bounds fail. Extracted encoder is tested independently of repository writes. |
| `raw/raw_detector.go`: `NewImageFromBuffer`, dimensions, `ToGoImage` | Removed the arbitrary 4 KiB-after-EOI rejection. Forces actual pixel decode rather than accepting a readable header. The JPEG signature remains an explicit embedded-preview format check, not a loader capability guess. |
| `raw/raw_processor.go`: preview normalization and full-render encoding through imaging | Removed the quality=100/format=0 passthrough: it bypassed the declared default encoder and metadata stripping. LibRaw remains the RAW decoder; rendered TIFF and embedded preview are explicitly re-encoded. |
| `raw/raw_processor.go`: `NewImageFromBuffer` in `populatePreviewDimensions` | Retained: reads dimensions of the already normalized/encoded result; it does not infer loader properties. |
| `raw/raw_processor.go`: `GenerateThumbnails` through imaging | Invalid bounds and failed encodes now return errors instead of a success result with missing sizes. Smart crop and configured output format remain explicit options. |
| `imagesource/imagesource.go`: model option construction | Retained after tracing semantic/BioCLIP to ML helpers and OCR/face to display-oriented thumbnail processing. No direct native load call or loader-specific option is present here. |
| `imaging/vips_runtime.go`: logging, `Startup` | Initialization error is retained and returned to `app.Run`; startup cannot silently continue without its native runtime. Process-wide once initialization, explicit concurrency and cache policy retained. |

## Upload precheck

`StringsJSONParam` intentionally returns nil for an empty optional filter.
Precheck instead needs a membership set: its previous nil-to-empty-string
conversion passed invalid JSON to `json_each`. It now supplies `[]` for an
empty full/quick group. The general optional-filter helper and SQL contract
are unchanged.

The HTTP handler regression uses a real SQLite database with an existing
asset, exercising full-only, quick-only, mixed, unsupported quick version,
and size mismatch. It verifies candidate identity and preserves
`duplicate=false`: preflight must not authorize skipping server verification.

## Evidence and limits

- `task server:test` passed on the final implementation. `task dto` completed
  with no generated changes; the public API shape is unchanged.

- Deliberate red runs reproduced full-only/quick-only precheck HTTP 500,
  ignored ML orientation, negative dimensions, rectangular crop failure,
  non-JPEG export orientation loss, face crop coordinate mismatch, rejection
  of a decodable preview, encoder bypass and swallowed thumbnail errors.
- Synthetic fixtures are generated in tests. They cover JPEG EXIF and its
  WebP/PNG transcodes, truncated entropy data, trailing data, dimensions,
  image decode and native startup failure in an isolated process.
- Real UHDR execution is verified locally against the shipped library. It is
  not yet an automatic UHDR fixture gate on every platform. No private media
  was added to the repository.
- This audit covers application govips calls; it does not claim exhaustive
  codec coverage, redesign RAW container extraction, or qualify all native
  dependency combinations. Existing RAW format signatures remain advisory
  inputs to LibRaw processing, not authoritative decoder capability tests.

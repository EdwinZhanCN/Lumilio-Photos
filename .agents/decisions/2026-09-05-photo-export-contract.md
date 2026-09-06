# Decision: One photo export contract for Fullscreen and Studio

Status: implemented — included in the beta.2 scope; both entry points share the
export dialog and settings while retaining their own image-generation path.

## Problem

Fullscreen and Studio exposed different defaults, sizes, file names and failure
behavior. Fullscreen closed its dialog even after a failed request, and Studio's
long-edge setting bounded the inner photo rather than the final framed image.

## Decision

- `lib/photo-export` owns format, quality, size, naming and browser download
  primitives. `PhotoExportDialog` receives plain source facts and an async export
  callback; it owns transient submission state, failure presentation and download
  handoff. No feature imports another feature's renderer or private UI.
- Both flows default to JPEG at 92%, maximum available dimensions, and an editable
  `<source>-lumilio` file name. The actual selected format determines the extension.
  A mismatched output MIME type is a failure, not a silently mislabeled file.
- Fullscreen transcodes the unedited source through the existing Server endpoint.
  Studio snapshots its current adjustments and composition, renders through its
  Worker, and does not implicitly save its sidecar. Neither path modifies the
  original. Original-file download remains a separate action without conversion.
- Size choices never upscale the source. Studio applies them to the final crop,
  rotation and canvas treatment, with a warning when runtime limits reduce the
  result. Dimension previews are estimates; RAW embedded previews and GPU limits
  can constrain what the original asset's catalog dimensions suggest.
- Formats are executor capabilities: Server supports JPEG, PNG, WebP and AVIF;
  Studio supports JPEG, PNG and WebP. Metadata preservation is best-effort and
  capability-specific: the Server retains compatible source metadata and color
  profiles; Studio copies descriptive EXIF/GPS for JPEG and WebP, corrects
  orientation/dimensions, and warns if copying fails. Studio PNG does not copy
  source metadata. The dialog explains these limits rather than promising identical
  metadata bytes across encoders.
- Export errors keep settings open for retry. Busy submissions lock the form and
  prevent duplicate requests. Closing/unmounting aborts request or download
  delivery; a running Studio render may finish without delivering a file. Success
  means browser download handoff, not proof of a completed disk write.
- `AssetActionsDialog` owns the viewer's Studio, album, similar-photo, share,
  original-download and reprocessing actions. `AssetExportDialog` owns only the
  photo-export adapter. Old Studio export UI and the obsolete export hook,
  including its unused bulk-export API, are removed rather than retained as
  compatibility paths.

## Alternatives considered

- Reuse the entire Studio pipeline for Fullscreen: rejected because ordinary
  conversion would acquire editor/GPU dependencies and working-resolution limits.
- Move edited-image rendering into the Server: rejected because the Server has no
  equivalent Studio composition engine; that is a separate rendering project.
- Share only field styling: rejected because defaults, file names and failure
  behavior would continue to diverge behind apparently identical controls.
- Force identical formats or metadata bytes everywhere: rejected because the two
  encoders have different real capabilities. A common policy with explicit limits
  preserves AVIF on the Server without pretending Canvas can encode it.

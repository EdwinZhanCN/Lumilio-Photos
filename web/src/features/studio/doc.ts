/**
 * # Studio
 *
 * Studio owns the authenticated `/studio` workspace for non-destructive photo
 * editing. It selects an existing photo, edits sidecar instructions, renders a
 * preview, and exports a new file without changing the preserved original.
 *
 * ## State
 *
 * {@link StudioEditMvp} is a route-local state machine between
 * {@link StudioHome}, a photo-only {@link PhotoPicker}, and
 * {@link StudioEditor}. An `assetId` query parameter opens an edit directly.
 *
 * {@link StudioEditor} owns the active session: asset metadata, normalized
 * adjustments, undo history, preview URLs, and save/export status.
 * {@link useComposition} separately owns the canvas treatment, layers,
 * template previews, and logo rasterization. Durable instructions combine
 * {@link StudioEditAdjustments} and {@link StudioComposition} in
 * {@link LumilioSidecarV1}.
 *
 * Recent-edit shortcuts are refresh-safe convenience state stored under
 * {@link STUDIO_RECENT_EDITS_KEY}. {@link readRecentEdits},
 * {@link recordRecentEdit}, and {@link clearRecentEdits} retain only asset
 * identity, display metadata, and timestamps; losing them does not lose the
 * sidecar or original media.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart LR
 *     ROUTE["/studio"] --> SHELL["StudioEditMvp"]
 *     SHELL --> HOME["StudioHome"]
 *     SHELL --> PICKER["PhotoPicker"]
 *     SHELL --> EDITOR["StudioEditor"]
 *     EDITOR --> WORKER["Render worker"]
 *     WORKER --> DEVELOP["DevelopEngine"]
 *     DEVELOP --> GEOMETRY["applyGeometry"]
 *     GEOMETRY --> COMPOSE["composeStudioImage"]
 *     EDITOR --> SIDECAR["LumilioSidecarV1"]
 *     EDITOR --> EXPORT["ExportPanel"]
 * ```
 *
 * Preview rendering transfers the visible canvas and source blob to a worker.
 * {@link DevelopEngine} applies pixel adjustments in WebGL2,
 * {@link applyGeometry} applies crop/rotation/flip, and
 * {@link composeStudioImage} draws the canvas treatment and layers in 2D.
 * Slider previews stay on the GPU; only export encodes a blob.
 *
 * {@link ExportPanel} selects format, quality, and output size.
 * {@link resolveExportSize} prevents upscaling and respects GPU limits, then
 * {@link preserveExif} copies compatible EXIF to the rendered output with an
 * upright orientation.
 *
 * ## Data
 *
 * Asset metadata, source media, EXIF, and sidecars come from authenticated
 * asset endpoints. A sidecar stores edit instructions, not rendered pixels:
 * adjustments transform the source, while composition is drawn around or on
 * top of that result.
 *
 * {@link CropOverlay} edits displayed geometry and
 * {@link mapRectDisplayedToSource} commits a source-pixel crop.
 * {@link estimateDepthField} optionally supplies a self-hosted WebGPU depth
 * field for layer occlusion; absence of WebGPU or the model simply disables
 * that effect. {@link applyTemplate} expands declarative frame templates into
 * the same canvas and layer data used by manual edits.
 *
 * Studio has no cross-feature public barrel. The application router owns its
 * entry, and worker, rendering, editor, crop, depth, frame, and export details
 * remain feature-private.
 *
 * @module
 */
import type PhotoPicker from "../assets/picker/index.ts";
import type { ExportPanel } from "./flows/editor/export/ExportPanel.tsx";
import type { CropOverlay } from "./flows/editor/crop/CropOverlay.tsx";
import type { StudioEditor } from "./flows/editor/StudioEditor.tsx";
import type { useComposition } from "./flows/editor/useComposition.ts";
import type { StudioHome } from "./flows/home/StudioHome.tsx";
import type { StudioEditMvp } from "./flows/workspace/StudioWorkspaceFlow.tsx";
import type {
  STUDIO_RECENT_EDITS_KEY,
  clearRecentEdits,
  readRecentEdits,
  recordRecentEdit,
} from "./state/recentEdits.ts";
import type {
  LumilioSidecarV1,
  StudioComposition,
  StudioEditAdjustments,
} from "./model/editTypes.ts";
import type { composeStudioImage } from "./modules/rendering/composeStudioImage.ts";
import type {
  mapRectDisplayedToSource,
  resolveExportSize,
} from "./modules/rendering/coordinateSystem.ts";
import type { DevelopEngine } from "./modules/rendering/developEngine.ts";
import type { applyGeometry } from "./modules/rendering/geometry.ts";
import type { preserveExif } from "./modules/export/exif.ts";
import type { estimateDepthField } from "./modules/depth/depthEstimation.ts";
import type { applyTemplate } from "./modules/frame/applyTemplate.ts";

export {};

/**
 * # Studio
 *
 * The Studio feature owns the authenticated `/studio` editing surface for
 * photos that already exist in the library. It provides a small route state
 * machine, a local recent-edit dashboard, the editor, sidecar save, and export.
 * It does not import new media, mutate album membership, or replace the asset
 * gallery; those remain in Upload, Collections, and Assets.
 *
 * ## The two halves of an edit
 *
 * An edit has two independent halves, and keeping them apart is the feature's
 * central idea.
 *
 * **Adjustments** ({@link StudioEditAdjustments}) transform the source pixels:
 * exposure, color, detail, crop, rotation, flip.
 *
 * **Composition** ({@link StudioComposition}) is drawn around and on top of the
 * developed result — a {@link CanvasSpec} border and a stack of
 * {@link Layer}s. It is never baked into the develop pipeline, so changing
 * exposure does not disturb a caption, and moving a caption does not re-run the
 * GPU pipeline.
 *
 * Both halves persist in one sidecar ({@link LumilioSidecarV1}). The
 * composition fields are additive and nullable, so a sidecar written before
 * they existed still reads as a valid v1 document.
 *
 * ## State
 *
 * {@link StudioEditMvp} is the route shell. It switches between three local
 * views: {@link StudioHome}, a shared {@link PhotoPicker}, and
 * {@link StudioEditor}. If the URL includes an `assetId` query parameter, the
 * shell opens the editor directly.
 *
 * Recent edits are client-local history stored under
 * {@link STUDIO_RECENT_EDITS_KEY}. {@link RecentEditRecord},
 * {@link readRecentEdits}, {@link recordRecentEdit}, and
 * {@link clearRecentEdits} persist only asset id, name, dimensions, and
 * timestamp; durable edit instructions live in the asset sidecar.
 *
 * {@link StudioEditor} owns the session: asset metadata, normalized
 * adjustments, undo history, preview URLs, and save/export flags. Composition
 * state lives in {@link useComposition}, which also owns template previews and
 * logo rasterization. The editor emits {@link StudioEditorActivity} so Studio
 * Home can update recent edits.
 *
 * ## Rendering
 *
 * All rendering runs in the feature worker. The main thread decodes the source
 * into image data, then sends `LOAD_IMAGE_DATA`, `RENDER_PREVIEW`,
 * `EXPORT_IMAGE`, and `SET_LOGOS`. The worker develops the photo on WebGPU,
 * WebGL2, WASM CPU, or Canvas 2D, then composes:
 * {@link composeStudioImage} applies {@link renderCanvasSpec} and
 * {@link drawLayers}. The worker is an implementation boundary, not a public
 * API.
 *
 * Geometry renders in the worker rather than as CSS on {@link Viewport},
 * because a frame drawn around the photo must rotate with it, not on top of it.
 *
 * Fonts load inside the worker through {@link ensureStudioFontsLoaded}, so text
 * is measured with the same context that draws it. Measuring in one place and
 * drawing in another is what makes alignment drift with line width.
 *
 * Logos cannot be rasterized in the worker — decoding SVG needs the DOM — so
 * {@link rasterizeLogos} runs on the main thread and transfers bitmaps across.
 *
 * ## Frames
 *
 * A {@link FrameTemplate} is declarative data: a canvas treatment plus anchored
 * elements. {@link applyTemplate} expands one into a canvas spec and ordinary
 * layers, after which nothing distinguishes template content from something the
 * user typed. Templates hold no rendering logic.
 *
 * {@link expandTemplate} is the only place template units are converted, and
 * resolves EXIF through {@link extractFrameExif} and brands through
 * {@link matchBrand}.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/studio"] --> SHELL["StudioEditMvp"]
 *     SHELL --> HOME["StudioHome"]
 *     SHELL --> PICKER["PhotoPicker"]
 *     SHELL --> EDITOR["StudioEditor"]
 *     HOME --> RECENT["recentEditsStore"]
 *     EDITOR --> TOP["TopBar"]
 *     EDITOR --> ASSET["AssetPanel"]
 *     EDITOR --> VIEW["Viewport"]
 *     EDITOR --> PANEL["EditorPanel"]
 *     PANEL --> DEV["DevelopSections"]
 *     PANEL --> FRAME["FramePanel"]
 *     PANEL --> TEXT["TextPanel"]
 *     EDITOR --> COMP["useComposition"]
 *     COMP --> TPL["applyTemplate"]
 *     EDITOR --> SIDE["LumilioSidecarV1"]
 *     EDITOR --> WORKER["studio edit worker"]
 * ```
 *
 * {@link TopBar} owns session commands. {@link AssetPanel} shows source
 * metadata and EXIF. {@link Viewport} owns fit/zoom, before preview, and render
 * errors. {@link EditorPanel} hosts the three tabs and the mobile bottom sheet;
 * {@link DevelopSections} renders the adjustment groups defined by
 * {@link DEVELOP_GROUPS}, {@link FramePanel} the presets and border, and
 * {@link TextPanel} the layer stack.
 *
 * ## Decisions
 *
 * Studio is non-destructive. The original asset stays preserved, saved edits
 * are sidecar instructions, and export downloads a new rendered file.
 *
 * Recent edits are convenience state only. Losing localStorage should remove
 * Studio Home shortcuts, not the saved sidecar or the original media.
 *
 * @module
 */
import type PhotoPicker from "@/features/assets/picker/index.ts";
import type { StudioEditMvp } from "./flows/workspace/StudioWorkspaceFlow.tsx";
import type { StudioHome } from "./flows/home/StudioHome.tsx";
import type {
  RecentEditRecord,
  STUDIO_RECENT_EDITS_KEY,
  clearRecentEdits,
  readRecentEdits,
  recordRecentEdit,
} from "./state/recentEdits.ts";
import type { StudioEditor, StudioEditorActivity } from "./flows/editor/StudioEditor.tsx";
import type { TopBar } from "./flows/editor/TopBar.tsx";
import type { AssetPanel } from "./flows/editor/AssetPanel.tsx";
import type { Viewport } from "./flows/editor/Viewport.tsx";
import type { EditorPanel } from "./flows/editor/EditorPanel.tsx";
import type { useComposition } from "./flows/editor/useComposition.ts";
import type { DEVELOP_GROUPS } from "./model/developConfig.ts";
import type { DevelopSections } from "./flows/editor/develop/DevelopSections.tsx";
import type { FramePanel } from "./flows/editor/frame/FramePanel.tsx";
import type { TextPanel } from "./flows/editor/text/TextPanel.tsx";
import type {
  LumilioSidecarV1,
  StudioComposition,
  StudioEditAdjustments,
} from "./model/editTypes.ts";
import type { CanvasSpec } from "./model/canvasSpec.ts";
import type { Layer } from "./model/layers.ts";
import type { composeStudioImage } from "./modules/rendering/composeStudioImage.ts";
import type { renderCanvasSpec } from "./modules/rendering/renderCanvas.ts";
import type { drawLayers } from "./modules/rendering/renderLayers.ts";
import type { ensureStudioFontsLoaded } from "./modules/rendering/fonts/loadStudioFonts.ts";
import type { applyTemplate } from "./modules/frame/applyTemplate.ts";
import type { expandTemplate } from "./modules/frame/expandTemplate.ts";
import type { FrameTemplate } from "./modules/frame/frameTemplate.ts";
import type { extractFrameExif } from "./modules/frame/frameExif.ts";
import type { matchBrand } from "./modules/frame/logoRegistry.ts";
import type { rasterizeLogos } from "./modules/frame/logoRaster.ts";

export {};

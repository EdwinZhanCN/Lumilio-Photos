/**
 * # Studio
 *
 * The Studio feature owns the authenticated `/studio` editing surface for
 * photos that already exist in the library. It provides a small route state
 * machine, a local recent-edit dashboard, the develop editor, sidecar save,
 * export, and the border tool. It does not import new media, mutate album
 * membership, or replace the asset gallery; those remain in Upload,
 * Collections, and Assets.
 *
 * ## State
 *
 * {@link StudioEditMvp} is the route shell. It switches between three local
 * views: {@link StudioHome}, a shared {@link PhotoPicker}, and
 * {@link StudioEditor}. If the URL includes an `assetId` query parameter, the
 * shell opens the editor directly. Otherwise the user starts from Studio Home,
 * chooses a photo, and can resume recent edits.
 *
 * Recent edits are client-local history stored under
 * {@link STUDIO_RECENT_EDITS_KEY}. {@link RecentEditRecord},
 * {@link readRecentEdits}, {@link recordRecentEdit}, and
 * {@link clearRecentEdits} persist only asset id, name, dimensions, and
 * timestamp; durable edit instructions live in the asset sidecar, not in
 * localStorage.
 *
 * {@link StudioEditor} owns the editor session state: loaded asset metadata,
 * normalized {@link StudioEditAdjustments}, undo history, preview URLs, before
 * preview, save/export flags, render errors, and border-result state. It emits
 * {@link StudioEditorActivity} back to the shell so Studio Home can update
 * recent edits. The default edit state is {@link DEFAULT_STUDIO_ADJUSTMENTS};
 * defaults are intentionally identity operations so an unchanged editor
 * represents the original image.
 *
 * ## Data
 *
 * The editor reads the asset record, sidecar, exported source image, and EXIF
 * record for the selected asset id. Sidecars use {@link LumilioSidecarV1}:
 * saving writes the adjustment instructions back to `/api/v1/assets/{id}/sidecar`
 * without overwriting the preserved original media.
 *
 * Preview and export rendering run through the feature worker file. The main
 * thread decodes the source into image data, then sends `LOAD_IMAGE_DATA`,
 * `RENDER_PREVIEW`, and `EXPORT_IMAGE` messages with request ids. The worker
 * chooses an engine such as WebGPU, WebGL2, WASM CPU, or Canvas 2D and returns
 * blobs for the preview/export path. The worker is an implementation boundary,
 * not a public feature API.
 *
 * Develop controls are defined by {@link DEVELOP_GROUPS} and rendered by
 * {@link DevelopPanel}. Geometry changes are tracked separately from numeric
 * photometric controls because preview rendering ignores rotation/flip while
 * {@link Viewport} applies them visually.
 *
 * The border tool is additive. {@link BorderPanel} edits border params; applying
 * a border first exports the current develop result, then runs
 * {@link runBorderTransform} through the shared worker client from
 * {@link useWorker}. EXIF-driven border modes use {@link extractBorderExif},
 * {@link hasSufficientExif}, {@link matchBrandKey}, and
 * {@link rasterizeBrandLogo}; users cannot manually edit EXIF or force a
 * camera logo.
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
 *     EDITOR --> DEVELOP["DevelopPanel"]
 *     DEVELOP --> BORDER["BorderToolSection / BorderPanel"]
 *     EDITOR --> SIDE["LumilioSidecarV1"]
 *     EDITOR --> WORKER["studio edit worker"]
 *     BORDER --> TOOL["runBorderTransform via useWorker"]
 * ```
 *
 * {@link TopBar} owns session commands such as back, undo, reset, before,
 * save, and export. {@link AssetPanel} shows source metadata and EXIF rows.
 * {@link Viewport} owns fit/zoom, before preview, rotation/flip presentation,
 * and render errors. {@link DevelopPanel} owns the grouped controls and mobile
 * bottom-sheet behavior.
 *
 * ## Decisions
 *
 * Studio is non-destructive. The original asset stays preserved, saved edits
 * are sidecar instructions, and export downloads a new rendered file.
 *
 * Border output is a derived result layered on top of develop adjustments.
 * Changing any develop control clears the border result because the previous
 * border no longer represents the current edit state.
 *
 * Recent edits are convenience state only. Losing localStorage should remove
 * Studio Home shortcuts, not the saved sidecar or the original media.
 *
 * @module
 */
import type PhotoPicker from "@/features/assets/picker/index.ts";
import type { useWorker } from "@/contexts/WorkerProvider.tsx";
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
import type { DEVELOP_GROUPS } from "./model/developConfig.ts";
import type { DevelopPanel } from "./flows/editor/develop/DevelopPanel.tsx";
import type {
  DEFAULT_STUDIO_ADJUSTMENTS,
  LumilioSidecarV1,
  StudioEditAdjustments,
} from "./model/editTypes.ts";
import type { BorderPanel } from "./modules/tools/border/BorderPanel.tsx";
import type {
  extractBorderExif,
  hasSufficientExif,
  matchBrandKey,
  rasterizeBrandLogo,
  runBorderTransform,
} from "./modules/tools/border";

export {};

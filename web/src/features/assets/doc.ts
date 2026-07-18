/**
 * # Assets
 *
 * The asset feature owns the main library timeline, trash timeline, reusable
 * browser surface, viewer inspection, selection, and export/bulk asset actions.
 * {@link Assets} is the ordinary `/assets` route; {@link AssetsTrash} scopes the
 * same gallery to deleted assets; collection/person/agent routes reuse
 * {@link AssetBrowser} with source-specific constraints or a pin source.
 *
 * ## State
 *
 * {@link useAssetBrowseRouteState} makes search, sort and applied filters URL-owned.
 * Page constraints are merged through {@link mergeAssetFilters} and cannot be
 * overridden by user parameters. {@link AssetBrowserScope} creates one scoped
 * Zustand store with {@link createAssetSelectionStore}; that store only holds
 * selected {@link BrowseItem} ids. Navigation helpers are exposed through
 * {@link useAssetBrowserNavigation}.
 *
 * Server state stays in TanStack Query hooks. Durable asset mutations are in
 * {@link useAssetActions}, and bulk commands resolve selection through
 * {@link useBulkAssetActions}; no fetched asset collection is mirrored into
 * the Zustand store.
 *
 * ## Structure
 *
 * `api/` contains server-state access and DTO adaptation; `model/` contains
 * React-free filtering, grouping, sorting, and browse-item rules. User journeys
 * are colocated under `flows/browse`, `flows/viewer`, and `flows/export`, so
 * workflow-specific components, hooks, state, tests, and styles have one owner.
 * Root `components/` is reserved for UI reused by multiple flows. Root `state/`
 * only holds the one-time persisted-state migration; it does not become a
 * second home for route state or server data.
 *
 * ## Data
 *
 * {@link useAssetBrowser} adapts explicit route state plus a page
 * constraint into an {@link AssetViewDefinition}, then reads
 * `/api/v1/assets/list` through {@link useAssetsList}. When search text is
 * present, it switches to `/api/v1/assets/search`
 * and returns the same {@link AssetsViewResult} shape.
 *
 * The rendering contract is {@link BrowseGroup} and {@link BrowseItem}, not raw
 * arrays of assets. {@link createBrowseGroupsFromBrowseItemDTOs},
 * {@link browseGroupsFromQueryLikePage}, and {@link flattenBrowseGroups} keep
 * ordinary assets and stacks in one flattened browse model so selection,
 * carousel positioning, and gallery tiles can share behavior.
 * Physical files are composed into logical media items before they reach this
 * browse surface: RAW/JPEG pairs and Live Photo still/video components render
 * once through their primary asset, while burst/manual presentation stacks
 * contain those logical items. {@link useAssetMediaItem} resolves components
 * for {@link MediaViewer}; {@link useStackCarouselAssets} resolves one primary
 * asset per logical stack member, so file counts never inflate burst counts.
 * In {@link AssetViewer}, the logical primary remains the Swiper item,
 * while RAW/JPEG selection is lifted into an active physical component that
 * drives metadata and asset-level actions without duplicating carousel slides.
 *
 * {@link usePinAssetsView} is the agent-board full-gallery adapter. It reads
 * `/api/v1/agent/pins/{id}/assets/list` and
 * `/api/v1/agent/pins/{id}/assets/search`, returning the same
 * {@link PinAssetsViewResult}/{@link AssetsViewResult} browse shape as library
 * views while constraining the backend query to the pin asset set. The older
 * `GET /api/v1/agent/pins/{id}/assets` hydration endpoint remains a lightweight
 * snapshot-order API for board previews.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["Assets / AssetsTrash routes"] --> PROVIDER["AssetBrowserScope"]
 *     PROVIDER --> PAGE["AssetBrowser"]
 *     PAGE --> HEADER["AssetsPageHeader"]
 *     PAGE --> JG["JustifiedGallery"]
 *     PAGE --> SG["SquareGallery"]
 *     PAGE --> FS["AssetViewer"]
 *     FS --> ACTIONS["AssetViewerActions"]
 *     ACTIONS --> EXPORT["AssetExportDialog"]
 *     PAGE --> SEARCH["SearchFAB"]
 *     PAGE -. pin .-> PIN["usePinAssetsView"]
 *     PAGE -. library .-> VIEW["useAssetBrowser"]
 *     VIEW --> BROWSE["BrowseGroup / BrowseItem"]
 *     PIN --> BROWSE
 *     BROWSE --> JG
 *     BROWSE --> SG
 *     BROWSE --> FS
 * ```
 *
 * {@link AssetBrowser} is the route orchestrator: it picks the source hook,
 * contributes visible selection to Lumilio context via
 * {@link useBrowseSelectionContext}, renders the chosen gallery layout, and
 * keeps URL-backed carousel navigation in sync.
 * {@link AssetsPageHeader} owns route-level controls; {@link JustifiedGallery}
 * and {@link SquareGallery} render the browse model; {@link AssetViewer}
 * inspects the current flattened asset set; {@link SearchFAB} writes debounced
 * search text to the URL and the selected source hook decides how to execute it.
 * {@link AssetViewer} owns carousel/media inspection and delegates mutation
 * dialogs and action state to {@link AssetViewerActions}; export and reprocess
 * behavior live in the separate export flow through {@link AssetExportDialog}.
 * {@link PhotoPicker} is the narrow cross-feature picker entry: it creates an
 * isolated single-selection asset scope while keeping gallery and filter
 * implementation details inside Assets.
 * Both galleries use {@link useGalleryViewportWindow}: the full layout height
 * remains stable, while only an overscanned vertical slice mounts thumbnail
 * components. Leaving that slice removes media nodes instead of retaining every
 * tile ever visited. Inactive list/search queries have a short bounded GC time.
 * {@link AssetPreviewGrid} is the finite dashboard-preview entry used outside
 * Assets; it hides browse-group conversion, gallery implementation, and its
 * isolated selection scope behind one public component.
 *
 * ## Decisions
 *
 * Browse items are the shared asset-set surface. Source adapters may all return
 * {@link AssetsViewResult}, but controls must remain capability-aware: library
 * and pin views can sort, filter, and search through source-scoped backend
 * queries, while repository scan remains a library maintenance action and is
 * hidden for pin/ref contexts.
 *
 * Selection stores browse item ids, not raw asset ids. Bulk actions call
 * {@link resolveBrowseSelectedAssetIds} so stacks can choose whether an action
 * affects the visible representative or every member.
 *
 * @module
 */
import type {
  AssetBrowserScope,
  useAssetBrowserNavigation,
} from "./flows/browse/selection/AssetBrowserScope.tsx";
import type { createAssetSelectionStore } from "./flows/browse/selection/selection.store.ts";
import type { mergeAssetFilters } from "./model/filter.ts";
import type { useAssetBrowseRouteState } from "./flows/browse/useAssetBrowseRouteState.ts";
import type { AssetViewDefinition, AssetsViewResult, BrowseGroup, BrowseItem } from "./types.ts";
import type {
  browseGroupsFromQueryLikePage,
  createBrowseGroupsFromBrowseItemDTOs,
  flattenBrowseGroups,
  resolveBrowseSelectedAssetIds,
} from "./model/browseItems.ts";
import type { useAssetActions } from "./api/useAssetActions.ts";
import type { useBulkAssetActions } from "./flows/browse/bulk-actions/useBulkAssetActions.ts";
import type { useAssetsList } from "./api/useAssetsList.ts";
import type { useAssetBrowser } from "./flows/browse/useAssetBrowser.ts";
import type { PinAssetsViewResult, usePinAssetsView } from "./api/usePinAssetsView.ts";
import type { AssetBrowser } from "./flows/browse/AssetBrowser.tsx";
import type { AssetPreviewGrid } from "./flows/browse/AssetPreviewGrid.tsx";
import type Assets from "./routes/Assets.tsx";
import type AssetsTrash from "./routes/AssetsTrash.tsx";
import type AssetsPageHeader from "./flows/browse/header/AssetsPageHeader.tsx";
import type JustifiedGallery from "./flows/browse/gallery/JustifiedGallery/JustifiedGallery.tsx";
import type SquareGallery from "./flows/browse/gallery/SquareGallery/SquareGallery.tsx";
import type AssetViewer from "./flows/viewer/AssetViewer.tsx";
import type { AssetViewerActions } from "./flows/viewer/AssetViewerActions.tsx";
import type { AssetExportDialog } from "./flows/export/AssetExportDialog.tsx";
import type { SearchFAB } from "./flows/browse/SearchFAB.tsx";
import type { useBrowseSelectionContext } from "./flows/browse/useBrowseSelectionContext.ts";
import type { useGalleryViewportWindow } from "./flows/browse/gallery/useGalleryViewportWindow.ts";
import type { useAssetMediaItem } from "./api/useAssetMediaItem.ts";
import type { useStackCarouselAssets } from "./api/useStackCarouselAssets.ts";
import type MediaViewer from "./flows/viewer/media/MediaViewer.tsx";
import type PhotoPicker from "./picker/PhotoPicker.tsx";

export {};

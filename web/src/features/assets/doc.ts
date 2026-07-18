/**
 * # Assets
 *
 * The asset feature owns the main library timeline, trash timeline, reusable
 * gallery shell, carousel inspection, selection, and bulk asset actions.
 * {@link Assets} is the ordinary `/assets` route; {@link AssetsTrash} scopes the
 * same gallery to deleted assets; collection/person/agent routes reuse
 * {@link AssetsGalleryPage} with source-specific filters or a pin source.
 *
 * ## State
 *
 * {@link AssetsProvider} creates one scoped Zustand store with {@link createAssetsStore}.
 * The store is intentionally UI-only: {@link createUISlice} holds sort, search
 * text and carousel route state; {@link createFiltersSlice} holds local filter
 * controls; {@link createSelectionSlice} holds selected {@link BrowseItem} ids.
 * Fine-grained readers live in `selectors.ts`, while navigation helpers are
 * exposed through {@link useAssetsNavigation}.
 *
 * Server state stays in TanStack Query hooks. Durable asset mutations are in
 * {@link useAssetActions}, and bulk commands resolve selection through
 * {@link useBulkAssetOperations}; no fetched asset collection is mirrored into
 * the Zustand store.
 *
 * ## Data
 *
 * {@link useCurrentAssetsView} adapts the scoped store state plus optional
 * route-level filters into an {@link AssetViewDefinition}, then reads
 * `/api/v1/assets/list` through {@link useAssetsView}. When search text is
 * present, {@link useCurrentAssetsSearchView} switches to `/api/v1/assets/search`
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
 * In {@link FullScreenCarousel}, the logical primary remains the Swiper item,
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
 *     ROUTE["Assets / AssetsTrash routes"] --> PROVIDER["AssetsProvider"]
 *     PROVIDER --> PAGE["AssetsGalleryPage"]
 *     PAGE --> HEADER["AssetsPageHeader"]
 *     PAGE --> JG["JustifiedGallery"]
 *     PAGE --> SG["SquareGallery"]
 *     PAGE --> FS["FullScreenCarousel"]
 *     PAGE --> SEARCH["SearchFAB"]
 *     PAGE -. pin .-> PIN["usePinAssetsView"]
 *     PAGE -. library .-> VIEW["useCurrentAssetsView"]
 *     VIEW --> BROWSE["BrowseGroup / BrowseItem"]
 *     PIN --> BROWSE
 *     BROWSE --> JG
 *     BROWSE --> SG
 *     BROWSE --> FS
 * ```
 *
 * {@link AssetsGalleryPage} is the route orchestrator: it picks the source hook,
 * contributes visible selection to Lumilio context via
 * {@link useGalleryContextContributor}, renders the chosen gallery layout, and
 * keeps URL-backed carousel navigation in sync.
 * {@link AssetsPageHeader} owns route-level controls; {@link JustifiedGallery}
 * and {@link SquareGallery} render the browse model; {@link FullScreenCarousel}
 * inspects the current flattened asset set; {@link SearchFAB} writes to the
 * shared search state and the selected source hook decides how to execute it.
 * {@link PhotoPicker} is the narrow cross-feature picker entry: it creates an
 * isolated single-selection asset scope while keeping gallery and filter
 * implementation details inside Assets.
 * Both galleries use {@link useGalleryViewportWindow}: the full layout height
 * remains stable, while only an overscanned vertical slice mounts thumbnail
 * components. Leaving that slice removes media nodes instead of retaining every
 * tile ever visited. Inactive list/search queries have a short bounded GC time.
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
import type { AssetsProvider } from "./AssetsProvider.tsx";
import type { createAssetsStore } from "./assets.store.ts";
import type { createUISlice } from "./slices/ui.slice.ts";
import type { createFiltersSlice } from "./slices/filters.slice.ts";
import type { createSelectionSlice } from "./slices/selection.slice.ts";
import type {
  AssetViewDefinition,
  AssetsViewResult,
  BrowseGroup,
  BrowseItem,
} from "./types/assets.type.ts";
import type {
  browseGroupsFromQueryLikePage,
  createBrowseGroupsFromBrowseItemDTOs,
  flattenBrowseGroups,
  resolveBrowseSelectedAssetIds,
} from "./utils/browseItems.ts";
import type { useAssetActions } from "./hooks/useAssetActions.tsx";
import type { useBulkAssetOperations } from "./hooks/useSelection.tsx";
import type { useAssetsNavigation } from "./hooks/useAssetsNavigation.ts";
import type {
  useAssetsView,
  useCurrentAssetsSearchView,
  useCurrentAssetsView,
} from "./hooks/useAssetsView.tsx";
import type { PinAssetsViewResult, usePinAssetsView } from "./hooks/usePinAssetsView.tsx";
import type { AssetsGalleryPage } from "./components/page/AssetsGalleryPage.tsx";
import type Assets from "./routes/Assets.tsx";
import type AssetsTrash from "./routes/AssetsTrash.tsx";
import type AssetsPageHeader from "./components/shared/AssetsPageHeader.tsx";
import type JustifiedGallery from "./components/page/JustifiedGallery/JustifiedGallery.tsx";
import type SquareGallery from "./components/page/SquareGallery/SquareGallery.tsx";
import type FullScreenCarousel from "./components/page/FullScreen/FullScreenCarousel/FullScreenCarousel.tsx";
import type { SearchFAB } from "./components/page/SearchFAB.tsx";
import type { useGalleryContextContributor } from "./hooks/useGalleryContextContributor.ts";
import type { useGalleryViewportWindow } from "./hooks/useGalleryViewportWindow.ts";
import type { useAssetMediaItem } from "./hooks/useAssetMediaItem.ts";
import type { useStackCarouselAssets } from "./hooks/useStackCarouselAssets.ts";
import type MediaViewer from "./components/shared/MediaViewer.tsx";
import type PhotoPicker from "./picker/PhotoPicker.tsx";

export {};

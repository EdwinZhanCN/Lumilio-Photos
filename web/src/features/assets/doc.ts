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
 *
 * {@link usePinAssetsView} is the agent-board adapter. It hydrates
 * `/api/v1/agent/pins/{id}/assets` into {@link PinAssetsViewResult}, which is
 * shaped like {@link AssetsViewResult} for rendering, but its source currently
 * supports only snapshot pagination. It does not consume the ordinary library
 * sort, filter, or search state.
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
 * inspects the current flattened asset set; {@link SearchFAB} only applies to
 * sources that support the ordinary library search query.
 *
 * ## Decisions
 *
 * Browse items are the shared asset-set surface. Source adapters may all return
 * {@link AssetsViewResult}, but controls must remain capability-aware: a library
 * view can sort, filter, search, and scan repositories; an agent pin/ref view is
 * a snapshot-hydration source unless the backend contract grows those query
 * semantics.
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
import type {
  useBulkAssetOperations,
} from "./hooks/useSelection.tsx";
import type { useAssetsNavigation } from "./hooks/useAssetsNavigation.ts";
import type {
  useAssetsView,
  useCurrentAssetsSearchView,
  useCurrentAssetsView,
} from "./hooks/useAssetsView.tsx";
import type {
  PinAssetsViewResult,
  usePinAssetsView,
} from "./hooks/usePinAssetsView.tsx";
import type { AssetsGalleryPage } from "./components/page/AssetsGalleryPage.tsx";
import type Assets from "./routes/Assets.tsx";
import type AssetsTrash from "./routes/AssetsTrash.tsx";
import type AssetsPageHeader from "./components/shared/AssetsPageHeader.tsx";
import type JustifiedGallery from "./components/page/JustifiedGallery/JustifiedGallery.tsx";
import type SquareGallery from "./components/page/SquareGallery/SquareGallery.tsx";
import type FullScreenCarousel from "./components/page/FullScreen/FullScreenCarousel/FullScreenCarousel.tsx";
import type { SearchFAB } from "./components/page/SearchFAB.tsx";
import type { useGalleryContextContributor } from "@/features/lumilio/contributors/useGalleryContextContributor.ts";

export {};

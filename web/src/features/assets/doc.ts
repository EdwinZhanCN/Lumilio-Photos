/**
 * # Assets
 *
 * Assets owns catalog and Trash browsing, reusable asset-set presentation,
 * selection, filtering, viewer inspection, and export/bulk asset actions.
 * Collection, People, Home, Share, Studio, and Lumilio reuse its reviewed
 * public surfaces instead of implementing another gallery or viewer.
 *
 * ## State
 *
 * {@link useAssetBrowseRouteState} makes search, sort, and applied filters URL
 * state. {@link FilterTool} keeps an unapplied draft in a local reducer and
 * commits normalized values to that route state. Page constraints are combined
 * through {@link mergeAssetFilters}; constrained fields remain locked and
 * cannot be removed by user controls.
 *
 * {@link AssetBrowserScope} creates one scoped Zustand selection store through
 * {@link createAssetSelectionStore}. It holds only selected {@link BrowseItem}
 * identities and viewer-navigation callbacks. Asset collections, metadata, and
 * mutation results remain TanStack Query server state. The root `state/`
 * directory contains only migration from the retired persisted browse state.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTES["Assets / Trash / scoped routes"] --> SCOPE["AssetBrowserScope"]
 *     SCOPE --> BROWSER["AssetBrowser"]
 *     BROWSER --> SOURCE["catalog or pin source"]
 *     BROWSER --> GALLERY["Justified / Square gallery"]
 *     GALLERY --> VIEWER["AssetViewer"]
 *     VIEWER --> EXPORT["AssetExportDialog"]
 *     VIEWER --> SPECIES["BioCLIP species reference"]
 *     PICKER["PhotoPicker"] --> SCOPE
 * ```
 *
 * {@link Assets} and {@link AssetsTrash} are thin route flows over
 * {@link AssetBrowser}; album, person, folder, tag, classifier, and pin
 * surfaces pass a constraint or source instead. {@link JustifiedGallery} and
 * {@link SquareGallery} share the same browse model and virtualize the mounted
 * viewport. {@link AssetViewer} keeps the logical primary as the carousel item
 * while allowing the active RAW/JPEG physical component to drive metadata and
 * actions.
 *
 * {@link AssetExportDialog} owns export and reprocess interaction.
 * {@link PhotoPicker} is the isolated single-selection entry used by other
 * features, while {@link AssetPreviewGrid} is the finite dashboard-preview
 * entry. Neither exposes gallery implementation details.
 *
 * ## Data
 *
 * {@link useAssetBrowser} reads `/api/v1/assets/list` through
 * {@link useAssetsList} and switches to `/api/v1/assets/search` when search is
 * active. Source adapters return {@link AssetsViewResult}; rendering consumes
 * {@link BrowseGroup} and {@link BrowseItem}. The conversion helpers, including
 * {@link createBrowseGroupsFromBrowseItemDTOs}, compose physical files into
 * logical media items before presentation.
 *
 * {@link useAssetMediaItem} resolves RAW/JPEG and Live Photo components.
 * {@link useStackCarouselAssets} resolves one logical primary per burst/manual
 * stack member, and {@link MediaCompositionBadges} distinguishes composition
 * from stacking. BioCLIP output is normalized by
 * {@link parseSpeciesPrediction}; {@link SpeciesReferenceTrigger} fetches a
 * localized reference only when opened.
 *
 * {@link usePinAssetsView} adapts pin-scoped list/search endpoints to the same
 * browse contract. Durable mutations live in {@link useAssetActions}; bulk
 * commands use {@link useBulkAssetActions} and
 * {@link resolveBrowseSelectedAssetIds} so stacks can deliberately target the
 * representative or all members. The root `index.ts`, `map/index.ts`, and
 * `picker/index.ts` are the only cross-feature runtime entries.
 *
 * @module
 */
import type { useAssetActions } from "./api/useAssetActions.ts";
import type { useAssetMediaItem } from "./api/useAssetMediaItem.ts";
import type { useAssetsList } from "./api/useAssetsList.ts";
import type { usePinAssetsView } from "./api/usePinAssetsView.ts";
import type { useStackCarouselAssets } from "./api/useStackCarouselAssets.ts";
import type { AssetBrowser } from "./flows/browse/AssetBrowser.tsx";
import type { AssetPreviewGrid } from "./flows/browse/AssetPreviewGrid.tsx";
import type FilterTool from "./flows/browse/filtering/FilterTool.tsx";
import type JustifiedGallery from "./flows/browse/gallery/JustifiedGallery/JustifiedGallery.tsx";
import type { MediaCompositionBadges } from "./flows/browse/gallery/media/MediaCompositionBadges.tsx";
import type SquareGallery from "./flows/browse/gallery/SquareGallery/SquareGallery.tsx";
import type { useBulkAssetActions } from "./flows/browse/bulk-actions/useBulkAssetActions.ts";
import type { AssetBrowserScope } from "./flows/browse/selection/AssetBrowserScope.tsx";
import type { createAssetSelectionStore } from "./flows/browse/selection/selection.store.ts";
import type { useAssetBrowser } from "./flows/browse/useAssetBrowser.ts";
import type { useAssetBrowseRouteState } from "./flows/browse/useAssetBrowseRouteState.ts";
import type { AssetExportDialog } from "./flows/export/AssetExportDialog.tsx";
import type Assets from "./flows/library/AssetsFlow.tsx";
import type AssetsTrash from "./flows/trash/AssetsTrashFlow.tsx";
import type AssetViewer from "./flows/viewer/AssetViewer.tsx";
import type { SpeciesReferenceTrigger } from "./flows/viewer/SpeciesReferenceTrigger.tsx";
import type { parseSpeciesPrediction } from "./flows/viewer/fieldGuide.ts";
import type {
  createBrowseGroupsFromBrowseItemDTOs,
  resolveBrowseSelectedAssetIds,
} from "./model/browseItems.ts";
import type { mergeAssetFilters } from "./model/filter.ts";
import type PhotoPicker from "./picker/PhotoPicker.tsx";
import type { AssetsViewResult, BrowseGroup, BrowseItem } from "./types.ts";

export {};

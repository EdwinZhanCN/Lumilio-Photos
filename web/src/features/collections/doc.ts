/**
 * # Collections
 *
 * The hub for every way of *grouping* assets that isn't the raw library
 * timeline: albums, places/trips, people, folders, tags, and utility views
 * (classifier albums, duplicates). {@link Collections} is the landing page —
 * rails that each preview a full route. Person *detail* lives in the
 * `people` feature; collections only owns the people rail/grid entry into it.
 * Folders sit alongside albums/places/people as their own hub rail — a
 * browsing concept, not a maintenance tool. Tags are reached through
 * {@link useUtilityShortcuts} alongside Duplicates and Trash, since it's a
 * browse-only utility-style view, not its own hub rail.
 *
 * ## State
 *
 * {@link CollectionsProvider} (read via {@link useCollections}) holds only the
 * feature's transient UI state — album multi-select and which edit/create modal
 * is open — reduced by {@link collectionsReducer} as {@link CollectionsAction}
 * over {@link CollectionsState}. Everything durable is server state in TanStack
 * Query; nothing fetched is mirrored here.
 *
 * ## Data
 *
 * Each rail has a distinct backend story, and the differences are the point:
 *
 * - **Albums** — a real backend entity. {@link useAlbums} paginates
 *   `/api/v1/albums`; {@link mapAlbumToUI} shapes each DTO for the grid.
 * - **Duplicates** — a backend-computed graph. {@link useDuplicateSummary},
 *   {@link useDuplicateGroupList} and {@link useDetectDuplicates} wrap
 *   `/api/v1/duplicates/*`.
 * - **Utility classifier albums** — not entities at all: {@link UTILITY_CLASSIFIERS}
 *   is a static client table of saved tag-source queries (documents, receipts,
 *   illustration) rendered as virtual albums over the asset list. **Liked**
 *   ({@link Liked}) is the same shape over `{ liked: true }` — no favorites
 *   table, just the existing `assets.liked` column filtered through the
 *   normal list/search endpoints.
 * - **Places / trips** — fully derived client-side. {@link useCityTrips} segments
 *   map points by geohashed city + time gaps into trips; there is **no backend
 *   trip entity**, so a trip is identity-less and editing it is meaningless.
 * - **Folders** — derived from `assets.storage_path` prefixes; there is no
 *   folders table. {@link useFolders} lists immediate child folders (recursive
 *   counts/covers) and {@link useFolderSummary} aggregates one folder path, both
 *   backed by new `/api/v1/assets/folders*` endpoints. Route identity is a
 *   `{ repositoryId, folderPath }` pair packed by {@link encodeFolderKey} /
 *   {@link decodeFolderKey} into an opaque `:folderKey` segment, since a raw
 *   path can contain slashes. Both queries exclude the app-managed
 *   `.lumilio/` and `inbox/` prefixes, so the rail only ever shows folders a
 *   human placed or scanned into the repository.
 * - **Tags** — a real vocabulary, but grouped by `(tag_id, source)` because the
 *   same tag name can carry both manual and AI/system assignments across
 *   different assets. {@link useTagSummaries} wraps the new
 *   `/api/v1/assets/tag-summaries` endpoint (counts/covers), distinct from the
 *   autocomplete-only `/api/v1/assets/tags` used by `@`-mentions. Route
 *   identity uses {@link encodeTagKey} / {@link decodeTagKey} over
 *   `{ tagName, source }`, matching the `tag_name` + `tag_source` filter pair
 *   `AssetFilterDTO` already supports.
 * - **Liked** — the utility rail ({@link useUtilityShortcuts}) also includes
 *   Liked alongside Duplicates and Trash. {@link Liked} scopes
 *   {@link AssetsGalleryPage} to `{ liked: true }` and hides the default
 *   `set-liked` bulk menu in favor of a single scoped "remove from Liked"
 *   action, since setting liked=true is meaningless on a page already
 *   filtered to liked assets.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     HUB["Collections · hub"]
 *     HUB --> UR["UtilitiesRail"] --> DUP["Duplicates"]
 *     UR --> UCA["UtilityClassifierAlbum"]
 *     UR --> TGS["Tags"] --> TD2["TagDetails"]
 *     HUB --> MR["MapRail"] --> TD["TripDetails"]
 *     HUB --> AR["AlbumRail"] --> AD["AlbumDetails · hero + edit"]
 *     HUB --> PR["PeopleRail"] -.->|people feature| PERSON["PersonDetails"]
 *     HUB --> FR["FoldersRail"] --> FD["FolderDetails"]
 * ```
 *
 * {@link Folders} and {@link Tags} are the list pages that route into
 * {@link FolderDetails} and {@link TagDetails}.
 *
 * {@link AlbumDetails}, {@link TripDetails}, {@link UtilityClassifierAlbum},
 * {@link FolderDetails} and {@link TagDetails} all render through the shared
 * {@link AssetsGalleryPage} orchestrator, differing only by injection points:
 * album scopes by `{ album_id }`, trip by `{ location(bbox), date }`, classifier
 * and tag detail by `{ tag_name, tag_source }`, folder detail by
 * `{ repository_id, folder_path, folder_recursive }`. Album detail carries an
 * *editable* hero — {@link CollectionHero} composed with {@link AlbumFormModal};
 * trips, classifier albums, and tag detail pass no `hero` and expose no edit,
 * because they have no entity to mutate. {@link FolderDetails} passes a `hero`
 * with breadcrumb/child-folder drilldown chips, not an edit surface — folders
 * still aren't a mutable entity. {@link Duplicates} is the one review-style
 * page, not an asset grid.
 *
 * ## Decisions
 *
 * Editing is modal-only — {@link AlbumFormModal} both creates and edits albums
 * over a shared modal shell, with no inline editing. Trips, classifier albums,
 * folders, and tags expose no edit affordance by design: they have no entity to
 * mutate (or, for tags, mutation is out of scope for v1 per the exec plan).
 *
 * @module
 */
import type { CollectionsProvider, useCollections } from "./CollectionsProvider.tsx";
import type { collectionsReducer } from "./collections.reducer.ts";
import type { CollectionsState, CollectionsAction } from "./collections.type.ts";
import type { useAlbums, mapAlbumToUI } from "./hooks/useAlbums.ts";
import type { useCityTrips } from "./hooks/useCityTrips.ts";
import type {
  useDuplicateSummary,
  useDuplicateGroupList,
  useDetectDuplicates,
} from "./hooks/useDuplicates.ts";
import type { UTILITY_CLASSIFIERS } from "./utils/utilityClassifiers.ts";
import type { useFolders, useFolderSummary } from "./hooks/useFolders.ts";
import type { useTagSummaries } from "./hooks/useTagSummaries.ts";
import type { encodeFolderKey, decodeFolderKey } from "./utils/folderKey.ts";
import type { encodeTagKey, decodeTagKey } from "./utils/tagKey.ts";
import type { useUtilityShortcuts } from "./components/utilityShortcuts.ts";
import type { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage.tsx";
import type { CollectionHero } from "@/components/collection";
import type Collections from "./routes/Collections.tsx";
import type AlbumDetails from "./routes/AlbumDetails.tsx";
import type TripDetails from "./routes/TripDetails.tsx";
import type UtilityClassifierAlbum from "./routes/UtilityClassifierAlbum.tsx";
import type Duplicates from "./routes/Duplicates.tsx";
import type Liked from "./routes/Liked.tsx";
import type Folders from "./routes/Folders.tsx";
import type FolderDetails from "./routes/FolderDetails.tsx";
import type Tags from "./routes/Tags.tsx";
import type TagDetails from "./routes/TagDetails.tsx";
import type { AlbumFormModal } from "./components/AlbumFormModal.tsx";
export {};

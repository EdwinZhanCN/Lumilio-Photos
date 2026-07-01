/**
 * # Collections
 *
 * The hub for every way of *grouping* assets that isn't the raw library
 * timeline: albums, places/trips, people, and utility views (classifier albums,
 * duplicates). {@link Collections} is the landing page — four rails, each a
 * preview that links to its own full route. Person *detail* lives in the
 * `people` feature; collections only owns the people rail/grid entry into it.
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
 *   illustration) rendered as virtual albums over the asset list.
 * - **Places / trips** — fully derived client-side. {@link useCityTrips} segments
 *   map points by geohashed city + time gaps into trips; there is **no backend
 *   trip entity**, so a trip is identity-less and editing it is meaningless.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     HUB["Collections · hub"]
 *     HUB --> UR["UtilitiesRail"] --> DUP["Duplicates"]
 *     UR --> UCA["UtilityClassifierAlbum"]
 *     HUB --> MR["MapRail"] --> TD["TripDetails"]
 *     HUB --> AR["AlbumRail"] --> AD["AlbumDetails · hero + edit"]
 *     HUB --> PR["PeopleRail"] -.->|people feature| PERSON["PersonDetails"]
 * ```
 *
 * {@link AlbumDetails}, {@link TripDetails} and {@link UtilityClassifierAlbum}
 * all render through the shared {@link AssetsGalleryPage} orchestrator, differing
 * only by injection points: album scopes by `{ album_id }`, trip by
 * `{ location(bbox), date }`, classifier by `{ tag_name, tag_source }`. Album
 * detail carries an *editable* hero — {@link CollectionHero} composed with
 * {@link AlbumFormModal}; trips and classifier albums pass no `hero` and expose
 * no edit, because they have no entity to mutate. {@link Duplicates} is the one
 * review-style page, not an asset grid.
 *
 * ## Decisions
 *
 * Editing is modal-only — {@link AlbumFormModal} both creates and edits albums
 * over a shared modal shell, with no inline editing. Trips and classifier albums
 * expose no edit affordance by design: they have no entity to mutate.
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
import type { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage.tsx";
import type { CollectionHero } from "@/components/collection";
import type Collections from "./routes/Collections.tsx";
import type AlbumDetails from "./routes/AlbumDetails.tsx";
import type TripDetails from "./routes/TripDetails.tsx";
import type UtilityClassifierAlbum from "./routes/UtilityClassifierAlbum.tsx";
import type Duplicates from "./routes/Duplicates.tsx";
import type { AlbumFormModal } from "./components/AlbumFormModal.tsx";
export {};

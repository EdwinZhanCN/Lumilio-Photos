/**
 * # Collections
 *
 * Collections owns grouped ways of browsing the catalog: albums, places,
 * people entry surfaces, folders, tags, Liked, duplicate review,
 * shared-link navigation, and classifier views. Person identity correction
 * remains in People; asset presentation remains in Assets.
 *
 * ## State
 *
 * Durable albums, folders, tag summaries, duplicate groups, and asset results
 * are TanStack Query server state. {@link AlbumsProvider} keeps only the album
 * flow's transient selection and modal interaction, reduced by
 * {@link albumsReducer}. Repository scope comes from Repositories; scoped
 * gallery search, filters, sort, and viewer identity remain URL state owned by
 * Assets.
 *
 * Place summaries are a transient projection of server location clusters;
 * they are not client-persisted entities. Utility classifiers are also derived
 * views. Folder and tag route identities use
 * {@link encodeFolderKey}/{@link decodeFolderKey} and
 * {@link encodeTagKey}/{@link decodeTagKey} so paths and composite identities
 * survive navigation without a second store.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     HUB["Collections hub"] --> ALBUMS["Albums"]
 *     HUB --> PLACES["Places / map"]
 *     HUB --> PEOPLE["People entry"]
 *     HUB --> FOLDERS["Folders"]
 *     HUB --> UTILITIES["Utilities"]
 *     UTILITIES --> TAGS["Tags / classifiers / liked"]
 *     UTILITIES --> DUPLICATES["Duplicate review"]
 *     ALBUMS --> BROWSER["AssetBrowser"]
 *     PLACES --> MAP["Assets map capability"]
 *     FOLDERS --> BROWSER
 *     TAGS --> BROWSER
 * ```
 *
 * {@link Collections} renders preview rails driven by the same
 * {@link useUtilityShortcuts} list used on the Utilities page.
 * {@link AlbumDetails}, {@link FolderDetails}, {@link TagDetails}, and
 * {@link UtilityClassifierAlbum} all compose {@link AssetBrowser} with
 * different constraints. Only album detail has an editable entity hero;
 * folders provide navigation, and tags/classifiers expose no entity editor.
 * {@link Duplicates} is a review workflow rather than an asset grid.
 *
 * ## Data
 *
 * {@link useAlbums} paginates real album entities.
 * {@link useDuplicateSummary}, {@link useDuplicateGroupList}, and
 * {@link useDetectDuplicates} wrap the duplicate graph and its maintenance
 * command. {@link useFolders}/{@link useFolderSummary} derive folder summaries
 * from repository storage paths, and {@link useTagSummaries} groups tag usage
 * by name and source.
 *
 * The Places rail derives city-level summaries from {@link useLocationClusters}
 * and links them to the map through URL-owned center and zoom parameters.
 * {@link UTILITY_CLASSIFIERS} defines saved classifier constraints, while
 * Liked is the ordinary asset browse contract constrained by `liked: true`.
 * Biological album detail can enqueue an album-scoped BioCLIP rebuild and
 * invalidates indexing coverage after acceptance. Only
 * {@link useDetectDuplicates} is exported from the root public entry because
 * Manage is its sole cross-feature consumer.
 *
 * @module
 */
import type { useAlbums } from "./api/useAlbums.ts";
import type {
  useDetectDuplicates,
  useDuplicateGroupList,
  useDuplicateSummary,
} from "./api/useDuplicates.ts";
import type { useFolderSummary, useFolders } from "./api/useFolders.ts";
import type { useTagSummaries } from "./api/useTagSummaries.ts";
import type AlbumDetails from "./flows/albums/AlbumDetailsFlow.tsx";
import type { AlbumsProvider } from "./flows/albums/state/AlbumsProvider.tsx";
import type { albumsReducer } from "./flows/albums/state/reducer.ts";
import type Duplicates from "./flows/utilities/DuplicatesFlow.tsx";
import type FolderDetails from "./flows/folders/FolderDetailsFlow.tsx";
import type Collections from "./flows/hub/CollectionsFlow.tsx";
import type TagDetails from "./flows/tags/TagDetailsFlow.tsx";
import type UtilityClassifierAlbum from "./flows/utilities/UtilityClassifierFlow.tsx";
import type { useUtilityShortcuts } from "./flows/utilities/useUtilityShortcuts.ts";
import type { decodeFolderKey, encodeFolderKey } from "./model/folderKey.ts";
import type { decodeTagKey, encodeTagKey } from "./model/tagKey.ts";
import type { UTILITY_CLASSIFIERS } from "./model/utilityClassifiers.ts";
import type { AssetBrowser } from "../assets/index.ts";
import type { useLocationClusters } from "../assets/map/useLocationClusters.ts";

export {};

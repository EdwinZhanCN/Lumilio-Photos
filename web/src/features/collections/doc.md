# Collections

The hub for every way of *grouping* assets that isn't the raw library
timeline: albums, places/trips, people, folders, tags, and utility views
(classifier albums, duplicates). [Collections](./routes/Collections.tsx) is the landing page —
rails that each preview a full route. Person *detail* lives in the
`people` feature; collections only owns the people rail/grid entry into it.
Folders sit alongside albums/places/people as their own hub rail — a
browsing concept, not a maintenance tool. Tags are reached through
[useUtilityShortcuts](./components/utilityShortcuts.ts) alongside Duplicates and Trash, since it's a
browse-only utility-style view, not its own hub rail.

## State

[CollectionsProvider](./state/CollectionsProvider.tsx) (read via [useCollections](./state/CollectionsProvider.tsx)) holds only the
feature's transient UI state — album multi-select and which edit/create modal
is open — reduced by [collectionsReducer](./state/reducer.ts) as [CollectionsAction](./types.ts)
over [CollectionsState](./types.ts). Everything durable is server state in TanStack
Query; nothing fetched is mirrored here.

## Data

Each rail has a distinct backend story, and the differences are the point:

- **Albums** — a real backend entity. [useAlbums](./api/useAlbums.ts) paginates
  `/api/v1/albums`; [mapAlbumToUI](./api/useAlbums.ts) shapes each DTO for the grid.
- **Duplicates** — a backend-computed graph. [useDuplicateSummary](./api/useDuplicates.ts),
  [useDuplicateGroupList](./api/useDuplicates.ts) and [useDetectDuplicates](./api/useDuplicates.ts) wrap
  `/api/v1/duplicates/*`.
- **Utility classifier albums** — not entities at all: [UTILITY_CLASSIFIERS](./utils/utilityClassifiers.ts)
  is a static client table of saved tag-source queries (documents, receipts,
  illustration) rendered as virtual albums over the asset list. **Liked**
  ([Liked](./routes/Liked.tsx)) is the same shape over `{ liked: true }` — no favorites
  table, just the existing `assets.liked` column filtered through the
  normal list/search endpoints.
- **Places / trips** — fully derived client-side. [useCityTrips](./hooks/useCityTrips.ts) segments
  map points by geohashed city + time gaps into trips. It explicitly drains
  both map-point and location-cluster pagination before claiming a complete
  result; there is **no backend
  trip entity**, so a trip is identity-less and editing it is meaningless.
- **Folders** — derived from `assets.storage_path` prefixes; there is no
  folders table. [useFolders](./api/useFolders.ts) lists immediate child folders (recursive
  counts/covers) and [useFolderSummary](./api/useFolders.ts) aggregates one folder path, both
  backed by new `/api/v1/assets/folders*` endpoints. Route identity is a
  `{ repositoryId, folderPath }` pair packed by [encodeFolderKey](./utils/folderKey.ts) /
  [decodeFolderKey](./utils/folderKey.ts) into an opaque `:folderKey` segment, since a raw
  path can contain slashes. Both queries exclude the app-managed
  `.lumilio/` and `inbox/` prefixes, so the rail only ever shows folders a
  human placed or scanned into the repository.
- **Tags** — a real vocabulary, but grouped by `(tag_id, source)` because the
  same tag name can carry both manual and AI/system assignments across
  different assets. [useTagSummaries](./api/useTagSummaries.ts) wraps the new
  `/api/v1/assets/tag-summaries` endpoint (counts/covers), distinct from the
  autocomplete-only `/api/v1/assets/tags` used by `@`-mentions. Route
  identity uses [encodeTagKey](./utils/tagKey.ts) / [decodeTagKey](./utils/tagKey.ts) over
  `{ tagName, source }`, matching the `tag_name` + `tag_source` filter pair
  `AssetFilterDTO` already supports.
- **Liked** — the utility rail ([useUtilityShortcuts](./components/utilityShortcuts.ts)) also includes
  Liked alongside Duplicates and Trash. [Liked](./routes/Liked.tsx) scopes
  [AssetsGalleryPage](@/features/assets/components/browse/AssetsGalleryPage.tsx) to `{ liked: true }` and hides the default
  `set-liked` bulk menu in favor of a single scoped "remove from Liked"
  action, since setting liked=true is meaningless on a page already
  filtered to liked assets.

## Composition

```mermaid
flowchart TD
    HUB["Collections · hub"]
    HUB --> UR["UtilitiesRail"] --> DUP["Duplicates"]
    UR --> UCA["UtilityClassifierAlbum"]
    UR --> TGS["Tags"] --> TD2["TagDetails"]
    HUB --> MR["MapRail"] --> TD["TripDetails"]
    HUB --> AR["AlbumRail"] --> AD["AlbumDetails · hero + edit"]
    HUB --> PR["PeopleRail"] -.->|people feature| PERSON["PersonDetails"]
    HUB --> FR["FoldersRail"] --> FD["FolderDetails"]
```

[Folders](./routes/Folders.tsx) and [Tags](./routes/Tags.tsx) are the list pages that route into
[FolderDetails](./routes/FolderDetails.tsx) and [TagDetails](./routes/TagDetails.tsx).

[AlbumDetails](./routes/AlbumDetails.tsx), [TripDetails](./routes/TripDetails.tsx), [UtilityClassifierAlbum](./routes/UtilityClassifierAlbum.tsx),
[FolderDetails](./routes/FolderDetails.tsx) and [TagDetails](./routes/TagDetails.tsx) all render through the shared
[AssetsGalleryPage](@/features/assets/components/browse/AssetsGalleryPage.tsx) orchestrator, differing only by injection points:
album scopes by `{ album_id }`, trip by `{ location(bbox), date }`, classifier
and tag detail by `{ tag_name, tag_source }`, folder detail by
`{ repository_id, folder_path, folder_recursive }`. Album detail carries an
*editable* hero — [CollectionHero](@/components/collection) composed with [AlbumFormModal](./components/AlbumFormModal.tsx);
trips, classifier albums, and tag detail pass no `hero` and expose no edit,
because they have no entity to mutate. [FolderDetails](./routes/FolderDetails.tsx) passes a `hero`
with breadcrumb/child-folder drilldown chips, not an edit surface — folders
still aren't a mutable entity. [Duplicates](./routes/Duplicates.tsx) is the one review-style
page, not an asset grid.

## Decisions

Editing is modal-only — [AlbumFormModal](./components/AlbumFormModal.tsx) both creates and edits albums
over a shared modal shell, with no inline editing. Trips, classifier albums,
folders, and tags expose no edit affordance by design: they have no entity to
mutate (or, for tags, mutation is out of scope for v1 per the exec plan).

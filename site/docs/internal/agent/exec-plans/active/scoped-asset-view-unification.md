# Scoped Asset View Unification

## Context

Five pages browse a *scoped stream of assets*: albums, classifier/smart albums,
trash, trips/places, and people. They should be one composition tree — the
`AssetsGalleryPage` orchestrator (state via `AssetsProvider`, header via
`AssetsPageHeader`, grid via `JustifiedGallery` + `FullScreenCarousel`, data via
`useAssetsView(baseFilter)`) — with their differences expressed through a small
set of injection points, not as five hand-assembled pages.

This is the frontend half of a long-standing design. The reusable hero pieces
(`CollectionTitle`, `MetaStatRow`) and the modal-only edit pattern
(`AlbumFormModal`) already exist; what's missing is the convergence onto the
orchestrator plus the two seams (`CollectionHero`, `viewOverride`).

## Current State (verified 2026-06-27)

| Page | Renders through | Hero | Status |
| --- | --- | --- | --- |
| Album details | `AssetsGalleryPage` | inline `CollectionTitle` + `MetaStatRow` | ✅ on orchestrator, hero not extracted |
| Classifier/smart album | `AssetsGalleryPage` | none | ✅ on orchestrator |
| Trash | assets feature view | none | tracked by `assets-feature-review.md` F4 |
| Trip/place details | **hand-rolled** (`AssetsProvider` + `AssetsPageHeader` + `useAssetsView` + gallery) | inline `CollectionTitle` + `MetaStatRow` | ❌ not converged |
| Person details (people feature) | hand-rolled | inline | ❌ not converged + backend-blocked |

So album and classifier views already share the orchestrator; trips and people
do not. No `CollectionHero` component exists yet — pages wire the title/stat
pieces by hand. The `bulkActions` / `hiddenBulkActions` injection point already
landed via `assets-bulk-actions.md`.

## The Six Injection Points

Everything else in the tree is identical; only these vary per page:

| Point | Meaning | Who sets it |
| --- | --- | --- |
| `baseFilter` | data scope, served by `/assets/list` (`AssetFilterDTO`) | 4 of 5 pages |
| `viewOverride` | replacement data source (own endpoint) | people only |
| `hero` | top info block | album / person only |
| `editModal` | edit affordance inside the hero | album / person |
| `bulkActions` | selection-toolbar action set | per page (DONE) |
| `search` toggle | whether scoped search is enabled | off for person |

### Target stability matrix

| Page | baseFilter | viewOverride | hero | editModal | search |
| --- | --- | --- | --- | --- | --- |
| Album | `{album_id}` | — | `CollectionHero` | `AlbumFormModal` | on |
| Classifier/smart | `{tag_name, tag_source}` | — | — | — | on |
| Trash | `{is_deleted:true}` | — | — | — | on |
| Trips | `{location(bbox), date}` | — | — | — | on |
| Person | — | `usePersonAssetsView` | `CollectionHero` | `PersonRenameModal` | off¹ |

¹ Person search is blocked: `AssetFilterDTO` has no `person_id`, so person uses a
`viewOverride` against `/people/{id}/assets/list` and disables scoped search.
Once the backend adds `person_id`, person switches to `baseFilter={person_id}`,
the `viewOverride` is deleted, and the table collapses to one shape.

## Goals

1. One orchestrator (`AssetsGalleryPage`) for all five scoped views.
2. Extract `CollectionHero` so album/person stop hand-wiring the hero.
3. Add a `viewOverride` seam so a page can supply its own data source when
   `AssetFilterDTO` can't express its scope (today: person only).
4. Keep the existing shared state contract: every view reads the same
   `useSortBy()` / `useSearchQuery()` from `AssetsProvider`, so sort/search
   controls stay in sync no matter who produces `browseGroups`.
5. Unblock person properly with a backend `person_id` filter, then remove the
   `viewOverride` for person.

## Implementation Checklist

### 1. Extract `CollectionHero`

- [ ] New `web/src/components/collection/CollectionHero.tsx`: composes
      `CollectionTitle` (+ optional mono code badge), `MetaStatRow`, and an edit
      button that opens a page-supplied `editModal`.
- [ ] Props shape (names may change): `title`, `code?`, `stats: MetaStat[]`,
      `cover?`, `editModal?: ReactNode` (or `onEdit` + render-prop).
- [ ] Export from `web/src/components/collection/index.ts`.

### 2. Album details → `CollectionHero`

- [ ] Replace `AlbumDetails`' inline title/stat assembly with `CollectionHero`,
      passing `AlbumFormModal` as `editModal`.
- [ ] No behavior change; this is the reference consumer.

### 3. Add the `viewOverride` seam to `AssetsGalleryPage`

- [ ] Add `viewOverride?` to `AssetsGalleryPageProps`: when present, the page
      uses it as the data source instead of `useAssetsView(baseFilter)`; sort and
      search still come from the provider.
- [ ] Keep `baseFilter` the default path; `viewOverride` is the escape hatch.

### 4. Trip details → orchestrator

- [ ] Migrate `TripDetails` to `AssetsGalleryPage` with
      `baseFilter={ location(bbox), date }`, no hero, default bulk actions.
- [ ] Delete the hand-rolled provider/header/gallery wiring once parity is
      confirmed.

### 5. Person details → orchestrator + seam

- [ ] Migrate `PersonDetails` (people feature) to `AssetsGalleryPage` with
      `viewOverride={usePersonAssetsView(personId)}`, `hero={CollectionHero}` +
      `PersonRenameModal`, `search` disabled.

### 6. Backend: `AssetFilterDTO.person_id` (unblocks the collapse)

- [ ] Add `person_id` to `AssetFilterDTO` and `/assets/list` filtering; run
      `make dto`.
- [ ] Switch person to `baseFilter={person_id}`, delete `usePersonAssetsView`
      and the `viewOverride`, re-enable scoped search. Five pages now differ only
      by the table above with no `viewOverride` column.

## Affected Files

- `web/src/components/collection/CollectionHero.tsx` (new) + `index.ts`
- `web/src/features/assets/components/page/AssetsGalleryPage.tsx` (viewOverride)
- `web/src/features/collections/routes/AlbumDetails.tsx`
- `web/src/features/collections/routes/TripDetails.tsx`
- `web/src/features/people/routes/PersonDetails.tsx`
- backend: `AssetFilterDTO` + assets list query + `make dto` (step 6)
- `web/src/features/collections/doc.ts` and `web/src/features/people/doc.ts`:
  update the Composition section once the tree actually converges (document the
  reality, not this target).

## Cross-plan Links

- `assets-bulk-actions.md` — already delivered the `bulkActions` /
  `hiddenBulkActions` injection point on `AssetsPageHeader` + `AssetsGalleryPage`.
- `assets-feature-review.md` F4 — owns the Trash view, another `baseFilter`
  consumer of the same tree.

## Validation

```bash
make web-test
```

Manual: album/trip/person/classifier/trash all browse with synchronized
sort+search, album/person show an editable hero, trips/classifier/trash show no
edit, and person search re-enables only after step 6.

## Open Questions

- `CollectionHero` API: render-prop `editModal` vs `onEdit` + controlled modal
  state owned by the page?
- Should `viewOverride` be a hook result or a plain `{ data, isLoading,
  fetchNextPage }` object, to keep `AssetsGalleryPage` agnostic to TanStack
  Query specifics?
- Is trip scope better expressed as a real `baseFilter` (bbox+date) end to end,
  or does it also need a `viewOverride` until the assets list supports bbox?

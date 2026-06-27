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

## Current State (completed 2026-06-27)

| Page | Renders through | Hero | Status |
| --- | --- | --- | --- |
| Album details | `AssetsGalleryPage` | `CollectionHero` + `AlbumFormModal` | ✅ converged |
| Classifier/smart album | `AssetsGalleryPage` | none | ✅ converged |
| Trash | assets feature view | none | tracked by `assets-feature-review.md` F4 |
| Trip/place details | `AssetsGalleryPage` (`baseFilter={ location(bbox), date }`) | none | ✅ converged |
| Person details (people feature) | `AssetsGalleryPage` (`baseFilter={ person_id }`) | `CollectionHero` + `PersonRenameModal` | ✅ converged |

All five scoped views now render through the `AssetsGalleryPage` orchestrator.
`CollectionHero` is extracted and consumed by album + person. The backend
`person_id` filter collapsed person onto a plain `baseFilter` —
`usePersonAssetsView` is deleted and person search is re-enabled. The
`viewOverride` seam was added in step 3 but, once person collapsed, had **zero
consumers**, so it was removed rather than left as a speculative escape hatch:
every scope is now expressible as an `AssetFilterDTO` `baseFilter`. The
`bulkActions` / `hiddenBulkActions` injection point landed earlier via
`assets-bulk-actions.md`.

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

- [x] New `web/src/components/collection/CollectionHero.tsx`: composes
      `CollectionTitle` (+ optional mono code badge), `MetaStatRow`, and an edit
      button that opens a page-supplied `editModal`.
- [x] Props shape (names may change): `title`, `code?`, `stats: MetaStat[]`,
      `cover?`, `editModal?: ReactNode` (or `onEdit` + render-prop).
- [x] Export from `web/src/components/collection/index.ts`.

### 2. Album details → `CollectionHero`

- [x] Replace `AlbumDetails`' inline title/stat assembly with `CollectionHero`,
      passing `AlbumFormModal` as `editModal`.
- [x] No behavior change; this is the reference consumer.

### 3. Add the `viewOverride` seam to `AssetsGalleryPage`

- [x] Add `viewOverride?` to `AssetsGalleryPageProps`: when present, the page
      uses it as the data source instead of `useAssetsView(baseFilter)`; sort and
      search still come from the provider.
- [x] Keep `baseFilter` the default path; `viewOverride` is the escape hatch.

### 4. Trip details → orchestrator

- [x] Migrate `TripDetails` to `AssetsGalleryPage` with
      `baseFilter={ location(bbox), date }`, no hero, default bulk actions.
- [x] Delete the hand-rolled provider/header/gallery wiring once parity is
      confirmed.

### 5. Person details → orchestrator + seam

- [x] Migrate `PersonDetails` (people feature) to `AssetsGalleryPage` with
      `viewOverride={usePersonAssetsView(personId)}`, `hero={CollectionHero}` +
      `PersonRenameModal`, `search` disabled.

### 6. Backend: `AssetFilterDTO.person_id` (unblocks the collapse)

- [x] Add `person_id` to `AssetFilterDTO` and `/assets/list` filtering; run
      `make dto`.
- [x] Switch person to `baseFilter={person_id}`, delete `usePersonAssetsView`
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

## Resolved Questions

- `CollectionHero` API: settled on a single **`edit?: { onOpen, modal, label? }`**
  prop. The page owns the open/close state; the button trigger and the modal node
  always travel together so they can't drift apart (an earlier split of separate
  `onEdit` + `editModal` props had no enforced pairing). No render-prop.
  Trip-style scroll-collapse (`dense`) was dropped; the orchestrator renders the
  hero statically between header and gallery, matching album.
- `viewOverride` shape: while it existed it was a plain **`AssetsViewResult`** (the
  same object every view hook already returns), so the page needed no special
  casing. But once the backend `person_id` filter collapsed person onto a
  `baseFilter`, the seam had **zero consumers** and was removed — every scope is
  now an `AssetFilterDTO` `baseFilter`. If a future scope genuinely can't be
  expressed as a filter, re-add the seam then rather than carry it speculatively.
- Trip scope: expressed as a real **`baseFilter={ location(bbox), date }`** end to
  end. `AssetFilterDTO` already carried `location`/`date`, so no `viewOverride`
  was ever needed for trips.

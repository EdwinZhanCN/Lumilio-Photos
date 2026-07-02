# Liked Page

## Goal

Add a first-class Liked/Favorites collection page that behaves like every other
asset gallery route.

The app already supports per-asset `liked` state, bulk liked updates, liked
filtering, and a legacy `GET /api/v1/assets/liked` endpoint. This plan turns
liked assets into a stable utility destination under Collections instead of a
filter the user must reconstruct manually.

## Product Shape

Route:

```text
/collections/liked
/collections/liked/:assetId
```

Navigation:

- add a `Liked` utility shortcut beside Duplicates and Trash
- use a heart icon
- show count when cheap enough; count can be deferred

Behavior:

- render through `AssetsGalleryPage`
- base filter is `{ liked: true }`
- support sort, search, filters, selection, rating, add-to-album, download,
  share link, and delete-to-trash
- hide the bulk action that sets selected assets to `liked=true` because the
  page is already scoped to liked assets
- keep the action to unset liked visible

## Non-Goals

- No new favorite table.
- No separate backend list implementation.
- No distinction between "liked" and "favorite" in data. UI copy can say
  "Liked" or "Favorites", but the persisted field remains `assets.liked`.
- No smart ranking or memory generation.

## Current State

- `AssetFilterDTO` supports `liked`.
- Unified asset list/search SQL already filters liked assets.
- `AssetsPageHeader` already exposes bulk liked/unliked actions.
- Full-screen media info panels can toggle the current asset's liked state.
- A legacy `GET /api/v1/assets/liked` endpoint returns raw `AssetDTO` arrays, but
  this is not the desired surface for modern gallery pages because it bypasses
  `BrowseItem`, collapsed stacks, search, and shared pagination semantics.

## Backend Plan

No backend feature work is required for the basic page.

Do not use `GET /api/v1/assets/liked` for the route. Use existing:

```http
POST /api/v1/assets/list
POST /api/v1/assets/search
```

with:

```json
{
  "filter": {
    "liked": true
  }
}
```

Optional cleanup after the page ships:

- mark `GET /api/v1/assets/liked` as legacy/deprecated in docs/comments
- keep it for compatibility unless a separate API cleanup plan removes it

## Frontend Plan

### 1. Add Route

Create:

```text
web/src/features/collections/routes/Liked.tsx
```

Implementation pattern should mirror `UtilityClassifierAlbum` and `AssetsTrash`:

- `ErrorBoundary`
- `AssetsProvider`
- `WorkerProvider`
- `AssetsGalleryPage`
- breadcrumbs: Home -> Collections -> Utilities -> Liked
- `baseFilter={{ liked: true }}`
- `viewKey="collections:liked"`
- `basePath="/collections/liked"`
- `syncUrl`

Add routes in `web/src/routes/routes.tsx`:

```tsx
{
  path: "/collections/liked",
  element: <Liked />,
}
{
  path: "/collections/liked/:assetId",
  element: <Liked />,
}
```

### 2. Add Utility Shortcut

Update:

```text
web/src/features/collections/components/utilityShortcuts.ts
```

Add:

- key: `liked`
- route: `/collections/liked`
- icon: `Heart`
- tone: likely `accent` or `primary`
- title: `collections.utilities.liked.title`

### 3. Adjust Bulk Actions For Scoped Liked Page

The page should make unliking easy.

Current bulk action model has one `set-liked` menu with both liked/unliked
options. That can remain. If the UI reads awkwardly, add an optional scoped
action in `Liked.tsx`:

- id: `unlike-assets`
- label: `Remove from Liked`
- icon: `HeartOff` or `Heart`
- confirmation: selected assets will no longer appear on this page
- operation: existing bulk liked update with `liked=false`

If a custom action is added, hide the default `set-liked` menu on this page to
avoid duplicate controls.

### 4. i18n

Use extract-then-fill:

1. Add `t("collections.utilities.liked.title", "Liked")` in code.
2. Run:

```bash
cd web && vp exec i18next-cli extract
```

3. Fill generated zh values.
4. Verify:

```bash
cd web && vp exec i18next-cli status
```

Do not manually add translation keys before extraction.

### 5. Docs

Update `web/src/features/collections/doc.ts`:

- utility rail now includes liked, trash, duplicates, and classifier albums
- liked is a virtual asset gallery over `liked=true`

Regenerate `doc.md`.

## Validation

Frontend:

```bash
make web-test
```

Manual smoke:

- open `/collections/liked`
- verify only liked assets appear
- unlike one asset from full-screen view and confirm it leaves the page after
  refresh/invalidation
- bulk unlike selected assets
- sort, filter, search, open carousel, and close carousel
- verify `/assets` and other collection routes keep their filter state isolated

## Risks And Decisions

- **Route state isolation**: use a dedicated `AssetsProvider` scope so liked
  filters do not pollute the main gallery.
- **Duplicate liked controls**: if the default liked bulk menu feels redundant,
  replace it with a clear `Remove from Liked` custom bulk action on this page.
- **Naming**: persisted field is `liked`; product copy can choose Liked or
  Favorites. Keep code naming close to the existing field.
- **Legacy endpoint drift**: avoid building the page on `/assets/liked`; that
  endpoint does not match the modern browse item gallery contract.

## Critical Files for Implementation

- `web/src/features/collections/routes/Liked.tsx`
- `web/src/routes/routes.tsx`
- `web/src/features/collections/components/utilityShortcuts.ts`
- `web/src/features/assets/components/shared/AssetsPageHeader.tsx`
- `web/src/features/collections/doc.ts`

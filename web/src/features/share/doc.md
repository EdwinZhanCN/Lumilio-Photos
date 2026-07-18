# Share

Apple-iCloud-Link-style public sharing: pick a set of assets, an album, or
a person, and get a revocable, time-limited link that a recipient can open
without an account and without ever touching the authenticated app.

## Two audiences, two surfaces

- **Owner-facing**: [CreateShareLinkModal](./flows/create/CreateShareLinkModal.tsx) (create) and
  [SharedLinks](./flows/manage/SharedLinksFlow.tsx) (manage: revoke/extend/delete) run inside the
  authenticated app shell and use [useShareLinks](./api/useShareLinks.ts) against
  `/api/v1/share-links`.
- **Public**: [PublicShare](./flows/public/PublicShareFlow.tsx) is mounted at `/s/:token` (and
  `/s/:token/:assetId` for the lightbox) as a sibling of the gated route
  tree in `app/router/AppRouter.tsx` — not inside it — so a recipient is
  never redirected through first-run setup or forced to authenticate. It uses
  [usePublicShareView](./api/usePublicShareView.ts) against `/api/v1/public/shares/{token}` and
  never calls an authenticated endpoint.

## Why the public viewer doesn't reuse the normal gallery

The public API deliberately returns a minimal, de-sensitized asset shape
(id, type, dimensions, duration, taken_time — no owner_id, storage_path, or
filename) and has no filter/search/sort in v1. Reusing the app's
`BrowseGroup`/`BrowseItem`/`Asset` types and `JustifiedGallery` would mean
either widening those types to something a public page could safely see, or
faking fields just to satisfy them. [PublicShareGrid](./flows/public/PublicShareGrid.tsx) and
[PublicShareLightbox](./flows/public/PublicShareLightbox.tsx) are small, purpose-built components instead;
[shareUrls](./model/shareUrls.ts) builds token-scoped media URLs (no media-token query
param, unlike `assetUrls`).

## Creation entry points

[createShareSelectedBulkAction](./flows/create/shareBulkAction.tsx) is a reusable "Share selected" bulk
action wired into every gallery that supports multi-select (Assets, Liked,
Album, Person, Utility classifier) — it opens [CreateShareLinkModal](./flows/create/CreateShareLinkModal.tsx)
with `sourceKind: "asset_snapshot"`. Album and Person detail pages also get
a whole-collection "Share" button in their `CollectionHero` `actions` slot,
using `sourceKind: "album"` / `"person"` with `sourceRef` — the backend
resolves the snapshot server-side, so the frontend never materializes a
large asset ID array for those. The backend also supports `utility_query`
and `pin` source kinds, but v1 has no dedicated button for them (reachable
today via select-all + "Share selected").

## Tokens are hash-only

The server stores only an HMAC of the share token, never the raw value —
so a share's URL can only ever be copied once, in
[CreateShareLinkModal](./flows/create/CreateShareLinkModal.tsx)'s success state, right after creation.
[SharedLinks](./flows/manage/SharedLinksFlow.tsx) intentionally has no "copy" action on existing rows;
the only recovery path for a lost link is revoking it and creating a new
one.

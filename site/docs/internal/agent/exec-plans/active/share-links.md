# Share Links

## Goal

Build a first-class Share Link system modeled after Apple iCloud Link: a
lightweight, revocable, time-limited public link for a selected asset set.

The first milestone should make it easy to share a small set of photos, an
album, a person page, or a utility/query result without requiring recipient
accounts and without exposing the authenticated Lumilio app.

## Product Decision

Implement Share Links before collaborative shared albums.

Apple's UX separates temporary link sharing from long-lived collaborative shared
albums. Lumilio should follow that split:

- **Share Link**: public-by-token, read-only, default expiry, explicit download
  permission, owner can stop sharing.
- **Shared Album**: invited users, comments, likes, uploads, activity, and
  subscriber management. This is out of scope for the first milestone.

Cloudflare Tunnel, WebDAV, reverse proxies, and custom domains are transport
options only. They are not part of this plan. This plan assumes the app already
has a reachable base URL and focuses on the share permission model, APIs, and
viewer.

## User Experience

### Create Link

Expose a `Share` action from:

- selected assets in the main gallery
- album detail
- person detail
- utility classifier pages
- liked/favorites page once that page exists

The create dialog should be compact:

- title, defaulted from the source context
- expiry, default `30 days`
- allow download, default `false`
- include originals, visible only when download is enabled, default `false`
- copy link primary action

Do not ask the user to understand network exposure in this dialog. The dialog
creates a share URL for the configured public base URL. If no public base URL is
configured, show a local/LAN URL and a short note that recipients must be able to
reach this machine.

### Public Viewer

The public share viewer must not use the authenticated app shell:

- no sidebar
- no navbar
- no settings
- no upload
- no authenticated API calls
- no Lumilio chat dock

Viewer layout:

- top bar with share title, asset count, expiry state, and download action when
  allowed
- dense gallery grid using the existing browse item rendering where practical
- full-screen media viewer with previous/next navigation
- expired/revoked/not-found states with no asset metadata leakage
- light footer attribution: `Shared with Lumilio Photos`

### Manage Links

Add an authenticated `Shared Links` management page:

- active, expired, revoked tabs or segmented control
- source type and title
- asset count
- created time
- expiry time
- last viewed time
- view count
- copy link
- stop sharing
- extend expiry
- delete record, only for expired/revoked rows

## Non-Goals

- No Cloudflare Tunnel automation.
- No WebDAV.
- No collaborative albums.
- No invitee accounts.
- No comments, likes, uploads, or activity feed on public shares.
- No public access to the normal `/api/v1/assets/*` authenticated media routes.
- No permanent public URLs for originals unless download is explicitly enabled
  for the share.

## Current State

Relevant existing pieces:

- Authenticated public routes live in `web/src/routes/routes.tsx`, while the app
  shell is applied only to protected `appRoutes`.
- Asset galleries already converge on `AssetsGalleryPage`, `AssetsViewResult`,
  `BrowseGroup`, and `BrowseItem`.
- Agent pins already introduced the useful idea of source-scoped asset queries:
  a source is resolved first, then normal list/search/filter/sort operates within
  that source.
- Media routes currently authorize thumbnails/originals through normal asset
  auth and media tokens. Share media must be separate so a share token never
  becomes a normal bearer/media token.
- Album, people, utility classifier, trash, and pin pages already prove the
  gallery can be mounted with source-specific constraints.

## Data Model

Add a migration for share links.

Recommended table:

```sql
CREATE TYPE share_link_status AS ENUM ('active', 'revoked');
CREATE TYPE share_link_source_kind AS ENUM (
  'asset_snapshot',
  'album',
  'person',
  'utility_query',
  'pin'
);

CREATE TABLE public.share_links (
  share_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id integer NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  title text NOT NULL,
  description text,
  source_kind share_link_source_kind NOT NULL,
  source_ref text,
  source_filter jsonb,
  asset_ids uuid[],
  asset_count integer NOT NULL DEFAULT 0,
  allow_download boolean NOT NULL DEFAULT false,
  include_originals boolean NOT NULL DEFAULT false,
  status share_link_status NOT NULL DEFAULT 'active',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_at timestamptz,
  last_viewed_at timestamptz,
  view_count bigint NOT NULL DEFAULT 0
);
```

Indexes:

- `share_links(owner_id, created_at DESC, share_id DESC)`
- `share_links(token_hash)`
- `share_links(status, expires_at)`

Use a generated token only once, return the raw token only in the create
response, and store only a hash. The raw token is the public URL secret.

### Source Semantics

Support two source modes:

- **Snapshot source**: stores `asset_ids` at creation time. Use this for selected
  assets and for early implementations of query/person/utility shares when a
  stable snapshot is more predictable.
- **Live source**: stores `source_kind`, `source_ref`, and optional
  `source_filter`, then resolves assets at view time. Use this later for album
  public links if live album updates are desired.

First milestone should prefer snapshots because Apple-style iCloud Links feel
like "share these items", not "publish a mutable query".

## Backend Plan

### 1. Add DTOs

Create `server/internal/api/dto/share_dto.go`.

DTOs:

- `CreateShareLinkRequestDTO`
- `CreateShareLinkResponseDTO`
- `ShareLinkDTO`
- `ListShareLinksResponseDTO`
- `UpdateShareLinkRequestDTO`
- `PublicShareMetadataDTO`
- `PublicShareAssetListRequestDTO`
- `PublicShareAssetListResponseDTO`
- `PublicShareDownloadPolicyDTO`

Do not return `token_hash`.

### 2. Add Queries

Create `server/internal/db/repo/queries/share_links.sql`.

Required queries:

- create share link
- list owner share links
- get owner share link
- update owner share link settings
- revoke owner share link
- delete owner share link record
- get active public share by token hash
- increment public view counters

Run `cd server && sqlc generate` after adding queries.

### 3. Add Service

Create `server/internal/service/share_link_service.go`.

Responsibilities:

- generate high-entropy tokens
- hash tokens with a stable server-side pepper derived from existing secret
  material or a dedicated config value
- validate expiry and permissions
- resolve selected assets, album assets, person assets, utility query assets,
  and pin assets into snapshot asset IDs
- provide public metadata without leaking owner internals
- expose source-scoped list/search methods for the public viewer

Share links should reuse the internal asset set/source-scoped query model. Do
not fork a separate asset-list implementation for shares.

### 4. Add Authenticated Handler

Create `server/internal/api/handler/share_link_handler.go`.

Authenticated endpoints:

```http
POST   /api/v1/share-links
GET    /api/v1/share-links
GET    /api/v1/share-links/{id}
PATCH  /api/v1/share-links/{id}
POST   /api/v1/share-links/{id}/revoke
DELETE /api/v1/share-links/{id}
```

Rules:

- owner scoped; admins should not casually manage other users' links in v1
- delete should only remove expired or revoked records
- revoke must immediately disable public access
- extending expiry should be explicit and audited through `updated_at`

### 5. Add Public Handler

Public endpoints:

```http
GET  /api/v1/public/shares/{token}
POST /api/v1/public/shares/{token}/assets/list
GET  /api/v1/public/shares/{token}/assets/{assetId}/thumbnail
GET  /api/v1/public/shares/{token}/assets/{assetId}/web-video
GET  /api/v1/public/shares/{token}/assets/{assetId}/web-audio
GET  /api/v1/public/shares/{token}/assets/{assetId}/original
POST /api/v1/public/shares/{token}/download
```

Public access rules:

- token must exist, be active, and not be expired
- requested `assetId` must belong to the resolved share asset set
- thumbnails and web media are allowed for any valid share asset
- original downloads require `allow_download = true` and
  `include_originals = true`
- zip downloads require `allow_download = true`
- expired/revoked/not-found responses should be indistinguishable enough to
  avoid token probing feedback

Do not reuse normal media-token auth for public share media. The share token is
its own capability and must remain scoped to the share.

### 6. Wire Router And App

Add a `ShareLinkControllerInterface` to `server/internal/api/router.go`.

Register:

- authenticated routes under `/api/v1/share-links`
- public routes under `/api/v1/public/shares`

Initialize the service and handler in `server/app`.

Run `make dto` after handler annotations are correct.

## Frontend Plan

### 1. Create Feature

Create `web/src/features/share/`.

Suggested files:

- `routes/PublicShare.tsx`
- `routes/SharedLinks.tsx`
- `components/CreateShareLinkModal.tsx`
- `components/ShareLinkSettingsPanel.tsx`
- `components/PublicShareHeader.tsx`
- `hooks/useShareLinks.ts`
- `hooks/usePublicShareView.ts`
- `utils/shareUrls.ts`
- `doc.ts`

Generate `doc.md` from `doc.ts`.

### 2. Public Route

Add public routes outside the protected app shell:

```tsx
{
  path: "/s/:token",
  element: <PublicShare />,
}
{
  path: "/s/:token/:assetId",
  element: <PublicShare />,
}
```

The public share route should still get `QueryClientProvider`, i18n, theme, and
notifications, but it must not require `ProtectedRoute`, `PrimaryRepositoryGate`,
`WorkerProvider`, or the app shell.

Check `SetupGate` and `BootstrapGate` behavior. If those gates block public
share pages before first-run setup, keep that behavior. If they block valid
shares after normal setup due to auth assumptions, split public share routes
before the protected gates.

### 3. Create Share Link Modal

Integrate `CreateShareLinkModal` into:

- `AssetsPageHeader` bulk actions for selected browse items
- `AlbumDetails` hero/actions
- `PersonDetails` hero/actions
- `UtilityClassifierAlbum`
- later liked/favorites and folder/tag pages

Use the same modal shell pattern as existing collection/person edit modals.

### 4. Public Gallery Hook

Implement `usePublicShareView` similarly to `usePinAssetsView`:

- read public metadata
- call `/api/v1/public/shares/{token}/assets/list`
- return `AssetsViewResult`
- support pagination
- support carousel route state
- do not expose mutating actions
- do not show normal asset filter/search controls in v1 unless backend supports
  public-scoped search safely

First version can be browse-only with date order. Add search/filter later only
if the public endpoint remains source-scoped and leak-free.

### 5. Public Media URLs

Add share-specific URL helpers:

- thumbnail
- web video
- web audio
- original download
- zip download

Do not call `assetUrls.getThumbnailUrl`, because it adds normal media-token auth
for authenticated app media.

### 6. Manage Links Page

Add a protected route:

```tsx
{
  path: "/collections/shared-links",
  element: <SharedLinks />,
}
```

Expose it from the Utilities rail or a share-management action. Keep it dense:
table/list with copy, revoke, extend, delete.

## Security Requirements

- Store token hashes, never raw tokens.
- Tokens must be long enough to resist guessing.
- Public APIs must not expose storage paths, repository paths, owner IDs, EXIF
  raw JSON, sidecars, private tags, albums, people identities, or unrelated
  metadata in v1.
- Do not let public share endpoints call normal authenticated asset handlers
  without a share membership check.
- Every public media request must prove both token validity and asset membership.
- Cache headers should avoid long-lived caching for share metadata and original
  downloads. Thumbnails can use short private caching at most.
- Rate limiting is not currently a global app feature, but share endpoints
  should be designed so rate limiting can be added at the router/middleware
  layer later.
- Log share access with token hash/share ID, never raw token.

## API Contract Discipline

- OpenAPI annotations must reference concrete DTOs.
- Run `make dto`.
- Do not hand-edit `web/src/lib/http-commons/schema.d.ts`.
- Do not add frontend response casts around stale share DTOs.

## Validation

Backend:

```bash
cd server && sqlc generate
make dto
make server-test
```

Frontend:

```bash
make web-test
```

Manual smoke:

- create a share link from selected assets
- open `/s/:token` in a logged-out browser
- verify thumbnails render without bearer auth
- open carousel and navigate within the share only
- verify original download is hidden when disabled
- enable download and verify zip/original routes work
- revoke share and verify existing URL stops working
- expire share and verify public state matches revoked/not-found behavior
- ensure authenticated `/assets` and collection routes still use normal media
  auth

## Risks And Decisions

- **Snapshot vs live source**: choose snapshot for v1. It is more predictable and
  simpler to secure. Add live album links later if needed.
- **Public route placement**: current public routes still live inside app-level
  providers and gates. Verify gates do not accidentally require auth for
  `/s/:token`.
- **Media duplication**: public share media routes will look similar to existing
  asset media routes. Keep authorization separate even if file-serving helpers
  are shared.
- **Token handling**: raw tokens are only available at creation/copy time. If a
  user loses the URL, they can copy a reconstructed URL only if the system stores
  a non-secret public slug plus raw token encrypted. Prefer simpler v1 behavior:
  show/copy the public URL at creation and list pages only if the raw token is
  stored encrypted. If not storing encrypted tokens, management can still revoke
  but cannot re-copy old URLs. Decide before implementation.
- **Public search**: do not ship public search until source-scoped search is
  proven not to leak counts or candidates outside the share set.
- **Download cost**: zip downloads over many originals can be expensive. Cap the
  number of assets or stream carefully.

## Open Questions

- Should v1 store raw tokens encrypted to allow re-copying existing links from
  the management page, or should it only store hashes and require regenerating a
  link if the URL is lost?
- Should album shares be snapshots in v1, or should album public links be live
  from the beginning?
- Should public viewer expose asset filenames? Apple-style links usually make
  browsing easy, but filenames can leak personal information.
- Should share links inherit the current repository scope when created from a
  filtered gallery?

## Critical Files for Implementation

- `server/internal/api/router.go`
- `server/app/app.go`
- `server/internal/service/asset_browse_service.go`
- `web/src/routes/routes.tsx`
- `web/src/features/assets/components/shared/AssetsPageHeader.tsx`

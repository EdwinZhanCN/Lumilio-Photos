/**
 * # Share
 *
 * Apple-iCloud-Link-style public sharing: pick a set of assets, an album, or
 * a person, and get a revocable, time-limited link that a recipient can open
 * without an account and without ever touching the authenticated app.
 *
 * ## Two audiences, two surfaces
 *
 * - **Owner-facing**: {@link CreateShareLinkModal} (create) and
 *   {@link SharedLinks} (manage: revoke/extend/delete) run inside the
 *   authenticated app shell and use {@link useShareLinks} against
 *   `/api/v1/share-links`.
 * - **Public**: {@link PublicShare} is mounted at `/s/:token` (and
 *   `/s/:token/:assetId` for the lightbox) as a sibling of the gated route
 *   tree in `app/router/AppRouter.tsx` — not inside it — so a recipient is
 *   never redirected through first-run setup or forced to authenticate. It uses
 *   {@link usePublicShareView} against `/api/v1/public/shares/{token}` and
 *   never calls an authenticated endpoint.
 *
 * ## Why the public viewer doesn't reuse the normal gallery
 *
 * The public API deliberately returns a minimal, de-sensitized asset shape
 * (id, type, dimensions, duration, taken_time — no owner_id, storage_path, or
 * filename) and has no filter/search/sort in v1. Reusing the app's
 * `BrowseGroup`/`BrowseItem`/`Asset` types and `JustifiedGallery` would mean
 * either widening those types to something a public page could safely see, or
 * faking fields just to satisfy them. {@link PublicShareGrid} and
 * {@link PublicShareLightbox} are small, purpose-built components instead;
 * {@link shareUrls} builds token-scoped media URLs (no media-token query
 * param, unlike `assetUrls`).
 *
 * ## Creation entry points
 *
 * {@link createShareSelectedBulkAction} is a reusable "Share selected" bulk
 * action wired into every gallery that supports multi-select (Assets, Liked,
 * Album, Person, Utility classifier) — it opens {@link CreateShareLinkModal}
 * with `sourceKind: "asset_snapshot"`. Album and Person detail pages also get
 * a whole-collection "Share" button in their `CollectionHero` `actions` slot,
 * using `sourceKind: "album"` / `"person"` with `sourceRef` — the backend
 * resolves the snapshot server-side, so the frontend never materializes a
 * large asset ID array for those. The backend also supports `utility_query`
 * and `pin` source kinds, but v1 has no dedicated button for them (reachable
 * today via select-all + "Share selected").
 *
 * ## Tokens are hash-only
 *
 * The server stores only an HMAC of the share token, never the raw value —
 * so a share's URL can only ever be copied once, in
 * {@link CreateShareLinkModal}'s success state, right after creation.
 * {@link SharedLinks} intentionally has no "copy" action on existing rows;
 * the only recovery path for a lost link is revoking it and creating a new
 * one.
 *
 * @module
 */
import type CreateShareLinkModal from "./flows/create/CreateShareLinkModal.tsx";
import type PublicShareGrid from "./flows/public/PublicShareGrid.tsx";
import type PublicShareLightbox from "./flows/public/PublicShareLightbox.tsx";
import type PublicShare from "./flows/public/PublicShareFlow.tsx";
import type SharedLinks from "./flows/manage/SharedLinksFlow.tsx";
import type { useShareLinks } from "./api/useShareLinks.ts";
import type { usePublicShareView } from "./api/usePublicShareView.ts";
import type { shareUrls } from "./model/shareUrls.ts";
import type { createShareSelectedBulkAction } from "./flows/create/shareBulkAction.tsx";
export {};

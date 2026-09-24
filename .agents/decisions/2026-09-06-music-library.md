# Decision: Use a first-class local Music projection

Status: implemented

This record supersedes the provisional scope decision in
[2026-09-05-recordings-only-audio-rejected.md](2026-09-05-recordings-only-audio-rejected.md).

## Problem

Audio Assets were file and processing records, not a usable local music
library. Reusing the existing mixed-media Albums would conflate user-curated
asset collections with releases, while a recordings-only page would not
provide durable playlists, artist/release browsing, or continuous playback.

## Decision

Lumilio has an additive, owner-scoped Music projection. An AUDIO Asset remains
the stable track/file identity; Music owns effective metadata, ordered artist
credits, release albums, playlists, and transient playback sessions. User
corrections are field-level overrides with optimistic revisions, so metadata
reprocessing never rewrites original bytes or silently removes manual grouping.
Album artist relations retain an explicit extracted/manual source boundary.

Music tables are populated by the existing Asset metadata commit path and a
bounded startup backfill. They use the Catalog transaction boundary, preserve
legacy mixed-media Albums, keep playlist occurrences independent, and retain
soft-trash/hard-delete tombstones through existing Asset foreign-key behavior.
All HTTP reads and writes resolve the authenticated owner explicitly.

Playback is a bounded server snapshot with an expiry, separate from saved
playlists. The Web mounts one `MusicPlayerProvider` and dock in the application
shell, collects bounded snapshots before exposing an editable queue, and
arbitrates with Asset media through the
neutral `lib/media` coordinator. Music does not require Lumen, online catalog
accounts, or a second storage/import tree.

## Implementation evidence

- `000015_music_library.up.sql` adds tracks, overrides, releases, artists and
  ordered credits, playlists, playback sessions, and an FTS projection without
  modifying the baseline schema.
- Audio extraction preserves album artist, ordered credits, identifiers,
  release precision, edition, disc/track numbers, and compilation state; the
  projection sync is fenced inside `ApplyAssetExtractedMetadataTx`.
- Owner-scoped `/api/v1/music` handlers expose catalog correction, playlist,
  and playback-session operations. OpenAPI, TypeScript types, and Redoc are
  regenerated from the handlers.
- `web/src/features/music` owns browse/detail flows, playlist editing, the
  persistent player, i18n, and generated feature documentation. Existing Asset
  audio viewing uses the same neutral playback coordination event.
- `task architecture:check`, `task server:sqlc`, `task server:test`,
  `task web:test`, i18n coverage, and feature-doc generation pass for the
  implementation.

## Alternatives considered

### Keep AUDIO as recordings-only

Rejected because the requested product includes releases, artists, durable
playlists, and continuous navigation playback.

### Reuse legacy mixed-media Albums and `album_assets`

Rejected because those tables represent user collections and cannot express
release metadata, owner-scoped credits, or repeated playlist occurrences.

### Create a second music storage/import pipeline

Rejected because Asset storage, Locations, processing, likes, trash, and media
delivery already define file ownership and lifecycle. A projection at the
metadata commit boundary gives Music its domain without duplicating ingestion.

### Depend on an online catalog or Lumen for identity

Rejected because local browsing, correction, and playback must remain useful
offline and with optional ML disabled. External matching can be added later
without changing the local identity boundary.

## Local interaction and artwork follow-up — 2026-09-07

The library retains its favorites/recent section across browse tabs. URL state
owns search, filters and pagination; detail links preserve the return context.
YesPlayMusic's library hierarchy informs the layout while the implementation
uses the existing React components and product theme.

The browse flow keeps that upper section and one page-level scroll coordinate
stable while switching views. Each catalog panel stays mounted and toggles its
visibility, so view changes do not discard panel-local UI state. URL search
parameters remain the source of truth for the active view and its filters;
they do not introduce a separate scroll cache per view.

Migration 16 adds owner-scoped album/artist favorites and revisioned local
plain-text lyrics. Track likes remain Asset-owned. An owner-keyed browser
snapshot restores queue IDs, cursor, seek and controls in a paused state;
media URLs are never persisted. Eager collection of the bounded session permits
safe queue editing without mixing server offsets with local removals. Lazy
paging with editable offsets was rejected because it can skip or duplicate
entries. Playlist occurrences retain distinct entry identities.

Audio artwork runs through existing VideoCodec extraction and ImageCodec scale
admission, followed by source-fenced derivative publication. Waveforms and square
covers coexist. Album lists hydrate artist credits and infer a live track cover
only when no explicit cover is set. Editing other album fields must not persist
an inferred cover. Missing embedded art retains a placeholder; originals are
never modified and no online artwork or lyrics source is required. Existing
tracks can use the existing derivative reprocessing operation.

Evidence: Server gate passes; Web gate passes 419 tests with 6 capability skips.
Focused regressions cover automatic cover derivation, artwork extraction and
missing art, execution admission, owner/revision checks, artist identity and
playback sorting. Browser verification includes preserved highlights, playlist
create/add, lyrics save/read, favorites filtering, queue visibility, paused seek
restoration and a 390px player/dialog layout. The demo's 31 embedded covers were
regenerated; all 35 waveforms remain. The remaining four OGG files contain only
audio streams. Generated API artifacts reproduce without content drift; the
Git-diff freshness gate remains nonzero while these changes are uncommitted.

# Portable asset sidecars: rating, liked, and user tags travel with the Repository

Status: active, created 2026-10-28. Nothing has landed. The owner decided the
format, the location, and the first-phase scope on 2026-10-28 (see
Decisions). It runs as steps SC0–SC4 of
[rc1-sequence.md](rc1-sequence.md). These come after the
[repository layout](repository-layout.md) plan, which creates the
`user/assets/` class and the only path constructors, and after the
[repository access](repository-access-simplification.md) plan, whose
fail-fast handles the drain uses. It lands before the
rc.1 tag ([rc-release.md](rc-release.md)). This plan revises one row of the
[storage command boundary](../../../.agents/decisions/2026-10-26-storage-command-boundary.md)
record. That row says lost catalog metadata returns only from a compatible
backup and that Lumilio promises nothing portable. After this plan, rating,
liked, and user tags become portable. Albums, People, Event names, and share
links still return only from a backup.

Goal: each asset's rating, liked flag, and user-added tags are written into
its Repository. They come back when a catalog creates that asset again, in
three cases:

- after the catalog is lost;
- when the Repository is opened or adopted by another install;
- when a copy of the Repository is registered.

This is not interoperability with other photo software, not two-way sync, and
not a second source of truth.

## Non-goals

- **XMP.** Lumilio does not read or write `.xmp` files and does not exchange
  data with Lightroom, darktable, or digiKam. A one-way XMP export can be
  proposed later on its own.
- **Other user data.** Albums, People names, Event titles, and music track
  overrides are candidates for a later phase, each needing its own owner
  decision. Share links are never portable, because they carry access tokens.
- **Read-only Repositories.** These are not supported
  ([writable storage](../../../.agents/decisions/2026-10-28-repositories-require-writable-storage.md)).
  A sidecar that cannot be written stays queued and is retried; it is never
  redirected to another location.
- **External edits.** Lumilio does not resolve sidecars edited by hand or by
  other software. They are Lumilio's private files under `.lumilio/`.

## Fixed contracts

- **The catalog is the truth.** A sidecar is written only after the catalog
  change commits. It is read only when the catalog creates an asset for an
  entry and holds no user facts for that asset. Once the asset exists, the
  catalog wins, and the next drain overwrites the sidecar.
- **Layout.** Each file entry has one sidecar,
  `.lumilio/user/assets/<entry path>.json`, which mirrors the entry's
  repository-relative path. It is built only by `layout.AssetSidecar`. Studio
  records live beside it in `user/studio/`.
- **Content is bound to bytes.** A sidecar records the entry's `content_id`.
  A reader applies it only when the content hash proves the match, the same
  rule as "proven content hash, size and source or nothing" for Studio in
  the storage command boundary record. A sidecar left over from a replaced
  file is ignored and is later rewritten or removed.
- **No owner field.** `repository_entries.asset_id` maps an entry to exactly
  one asset, and that asset has one owner. On read, the facts apply to
  whichever asset the catalog creates for the entry.
- **No write inside a catalog transaction, and no handle held across a
  wait.** These rules come from the
  [repository access plan](repository-access-simplification.md). Each write
  is a temp file plus an atomic rename inside the Repository.
- **Readers accept added fields.** Format 1 readers tolerate new fields, the
  same rule as the Trash sidecar (`trash.ParseSidecar`). A new format number
  is needed only for an incompatible change.

## Decisions (frozen 2026-10-28; extract at completion)

- **Format and location (owner):** Lumilio JSON in a path mirror under
  `.lumilio/user/assets/`, the `user` class of
  [repository layout](repository-layout.md). Rejected alternatives:
  - *Standard XMP next to the original.* It writes into the user's folders
    and collides with other software's sidecars (Lightroom writes
    `IMG.xmp`, darktable writes `IMG.jpg.xmp`). It needs Lightroom-style
    conflict detection and a choice UI, and Lumilio-specific fields map
    lossily.
  - *A file keyed by content hash.* A person cannot find it, and duplicate
    files at two paths would share one file.
  - *One manifest per Repository.* Every change rewrites one large file, and
    corrupting it loses everything.
- **Scope (owner):** `assets.rating`, `assets.liked`, and `asset_tags` rows
  with `source = 'user'`, stored by tag name. AI, system, BioCLIP, and
  zero-shot tags are derived data and are never stored.
- **Change capture uses catalog triggers.** In the same transaction as the
  change, triggers on `assets` (updates of `rating` and `liked`) and on
  `asset_tags` (inserts and deletes where `source = 'user'`) upsert
  `asset_sidecar_outbox(asset_id, revision)`. These writes come from at least
  eight query paths in `assets_00.sql`, `duplicates.sql`, and
  `repository_scan_index.sql`:
  - single and bulk rating and liked updates;
  - the liked toggle;
  - the rating carry-over in the scan index;
  - adding and removing tags;
  - duplicate-group merges.

  Enqueueing from service methods was rejected, because bulk, toggle, scan,
  and Agent paths already bypass the single-asset methods.
- **The drain follows the OCR index outbox.** It is woken from committed
  mutations and does not poll
  ([OCR outbox drain](../../../.agents/decisions/2026-08-17-ocr-index-outbox-drain-scheduling.md)).
  It is bounded, at least once, and idempotent: a repeated pass rewrites the
  same bytes. One pass writes the current facts to every present entry of the asset. It
  removes the sidecar of an entry that is gone and of an asset whose facts
  are now empty. While the Repository is offline or not writable, the row
  stays queued.
- **Duplicate entries.** Two paths with the same content share one asset, and
  each path gets the same facts. If they disagree on read, the newer
  `updated_at` wins.

## Execution phases

### Phase 0 — Lock the failures and the facts

- Add a storage test: open a Repository into a fresh catalog, and its assets
  lose rating, liked, and user tags. It records today's behavior, and Phase 3
  inverts it into the acceptance test.
- Classification is settled by the repository layout. Everything under `user/` is the
  `user` class: kept in place and reassociated only on proven content.
  Recovery, reconnect, and register-copy therefore preserve the mirror
  without a rule of their own.
- Path limits: the mirror adds a `.lumilio/user/assets/` prefix and a
  `.json` suffix. Check this against the volume rules
  ([volume semantics](../../../.agents/decisions/2026-10-26-repository-volume-semantics.md)):
  the 255-byte name limit and Windows path length. Decide the behavior when a
  mirror path cannot exist: skip the entry with a recorded reason, never
  truncate.

### Phase 1 — Format, writer, reader

- Add one package in the storage group. It marshals and parses format 1 and
  writes and deletes atomically through a `RepositoryFS`.
- Cover the round trip, tolerance of added fields, a mismatched
  `content_id`, and a torn temp file left behind by a crash.

### Phase 2 — Change capture and drain

- Add `asset_sidecar_outbox` and its triggers to the catalog baseline. Follow
  the baseline rules in the
  [rc compatibility baseline](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md):
  edit the baseline in place before rc.1, and add a forward migration after
  it.
- Add the drain worker under the work layer. It is bounded and woken on
  commit, and it holds no handle across waits.
- Add tests: each of the eight query paths enqueues the asset, and an offline
  Repository keeps its rows.

### Phase 3 — Read when an asset is created

- Scan and discovery apply a proven sidecar when they create an asset for an
  entry. A tag name resolves to a `tags` row (created if missing) with
  `source = 'user'`.
- Invert the Phase 0 test: a fresh catalog restores rating, liked, and user
  tags.

### Phase 4 — Lifecycle integration

- Move and rename inside a Repository: the drain rewrites the sidecar at the
  new path.
- Trash: a trashed entry keeps its sidecar until purge, and restore keeps the
  facts.
- Reconnect, rekey, register-copy, and adopt carry the sidecar tree through
  the private class from Phase 0.

### Phase 5 — Product copy, docs, decision

- Adopt and open disclosures currently promise nothing portable. Change them
  to say that ratings, likes, and tags come back, and that albums, People,
  Events, and share links still need a backup. Use the canonical terminology
  registry and extract-then-fill i18n.
- Update the user docs on the site, in `en/` and `zh-cn/`.
- Extract the decisions into a record that supersedes the 10-26 row, and
  cross-link the two.

## Validation boundaries

- Destroy the catalog, then open or adopt the same Repository on a fresh
  install. Every asset's rating, liked flag, and user tags equal what they
  were before. Extend the `storage-recovery` E2E slice for this.
- After a change through a single update, a bulk update, the liked toggle, or
  an Agent tool, the sidecar matches the catalog within one drain. A change
  made while the Repository is offline is written after it reconnects.
- A replaced original never inherits its predecessor's facts.
- No sidecar write runs inside a catalog transaction, and the drain holds no
  Repository handle across a wait.
- `task server:test`, `task atlas:check`, and the `storage-recovery` slice are
  green.

# Ownership And Repository Scope

## Goal

Give Lumilio a final, documented positioning for its two scoping axes —
**repository** (where media physically lives) and **ownership** (whose media
it is) — and fix every place where the code drifts from that positioning.

This plan builds on the current uncommitted Folders/Tags + browse-scope
changes (which implement `folders-tags.md`); land those first, then execute
the workstreams below on top.

## The Model

### Positioning against Immich and PhotoPrism

- **Immich**: everything is per-user. Assets, external libraries, people, and
  duplicate groups all carry an `ownerId`; clustering and duplicate detection
  run per owner; cross-user visibility exists only through explicit sharing
  (shared albums, partner sharing). Owners are a hard partition.
- **PhotoPrism**: one shared library, folder-centric, admin-centric. Storage
  layout (originals folders) is a first-class browse axis; per-user ownership
  is weak by design.
- **Lumilio (deliberate hybrid)**: PhotoPrism-style **shared repository
  registry** — repositories are admin-managed storage locations inside one
  unified library, carry no per-user ownership, and are union-friendly
  (a person/album/tag spanning repositories is correct) — combined with
  Immich-style **per-user ownership** (`assets.owner_id`) as the only real
  visibility/mutation boundary. `repositories.default_owner_id` is a fallback
  owner for newly discovered files (`internal/sourcing/materializer.go`), not
  an ACL.

### Two rules (both already proven in this codebase)

1. **Grouping entities (cluster, stack, duplicate group) must include the
   owner in their grouping key — structurally, not as an optional filter.**
   Template: `location_clusters` (`UNIQUE(owner_id, repository_id, geohash)`,
   owner baked into `GROUP BY`). Repository stays an optional, union-friendly
   read filter on these same entities; owner is a hard partition.
2. **Mutations on a grouped entity require whole-entity ownership or admin —
   never "owns ≥1 member".** Template: `getAuthorizedAsset` /
   `getAuthorizedAlbum`. Once rule 1 holds, each grouped entity has exactly
   one owner and this collapses to an equality check.

### The four sources of "which repository"

| # | Source | Lifetime | Used for |
|---|--------|----------|----------|
| 1 | Browse scope (`useBrowseScope`) | Sticky preference, read-only | What am I looking at? "All" is a valid default. List pages only; never feeds a mutation. |
| 2 | Working repository (`useWorkingRepository`) | Sticky, always resolves to one concrete repo (primary fallback) | Where does new content land? Upload only. |
| 3 | Entity-owned `repository_id` | Derived from data | Actions on one loaded entity that is structurally single-repo: asset, folder, duplicate group. |
| 4 | Explicit per-repo action target | One-shot, chosen by the clicked row | Maintenance jobs (scan, rebuild) in Manage. |

Entities that are **not** structurally single-repo — albums (`albums` has no
`repository_id`) and people (`face_clusters` has none) — take **no repository
filter at all** on their detail pages and mutations. Ownership is the only
boundary there.

## Verified Current State

Confirmed against code (2026-07-01):

- 🔴 **Duplicate groups**: `POST /duplicates/groups/:id/merge|dismiss` have
  only `AuthMiddleware` (`router.go:471`) and no owner check in
  `duplicate_handler.go` or the service; `queries/duplicates.sql` never
  references `owner_id`; `duplicate_groups` has no owner column. Any
  authenticated user can merge (soft-deleting assets!) or dismiss any other
  user's group.
- 🟠 **People**: automatic clustering is already partitioned by
  `faceClusterScope{RepositoryID, OwnerID, EmbeddingModel}`
  (`face_clustering.go:19`), but the cluster entity has no `owner_id` column,
  so: authorization is ANY-match (`GetPersonByIDScoped`'s `EXISTS` over
  member assets, `people.sql:104`); `GetClusterMergeCandidates`
  (`faces.sql:461`) has no owner parameter and can suggest cross-owner
  merges; manual merge/move can create mixed-owner clusters; `is_hidden` is a
  global column on the cluster row. Also: the clustering scope contains
  `RepositoryID`, so automatic people are silently per-repo — contradicting
  the "people span repositories" product intent.
- 🟡 **Asset stacks**: manual create/unstack/live-photo matching are
  owner-scoped in SQL, but `FindCandidatesForStackingByName`
  (`stacks.sql:92`) filters only by repository, and `GetAssetStack`
  (`asset_handler.go:4089`) authorizes only the requested asset then returns
  the full member list.
- 🟡 **Schema**: `location_clusters.owner_id` is `NOT NULL DEFAULT 0` with no
  FK to `users` (migration 000004:155), unlike every other owner column.
- 🟢 Direct-ownership entities (`assets`, `albums`, `agent_pins`,
  `cloud_credentials`) and tags are clean. `location_clusters` authorization
  behavior is the reference implementation.
- 🟢 The uncommitted diff already adds `ownerScopeID(c)` (`asset_authz.go`)
  and owner-scopes the new folder/tag summary queries; `useBrowseScope` /
  `useWorkingRepository` are split, and working repository now always
  resolves to primary.

## Non-Goals

- No sharing system (shared albums, partner sharing). Ownership stays a hard
  partition; cross-user viewing is admin-only for now.
- No per-user repositories. The registry stays admin-managed and unowned.
- No behavior change for single-user (desktop) installs — owner scoping
  degenerates naturally when there is one user.

## Workstream A — Backend Ownership Hardening

Ranked by severity. **Schema changes edit the original CREATE TABLE
migrations in place** (000004 for `duplicate_groups`/`asset_stacks`/
`location_clusters`, 000005 for `face_clusters`) — the project is
pre-production and dev environments are reset for end-to-end validation, so
no ALTER migrations, no backfill, and no squash debt. Owner stamps come from
detection/clustering running fresh.

### A1. Duplicate groups (authorization gap — do first)

1. Schema: add `owner_id integer REFERENCES users(id) ON DELETE CASCADE`
   (nullable) to the `duplicate_groups` CREATE TABLE in migration 000004,
   plus an `(owner_id, repository_id)` index.
2. Detection (`queries/duplicates.sql` + duplicate service): make owner part
   of the candidate grouping key — a group never spans owners — and stamp
   `owner_id` on insert. Detection per repository stays as-is (repo is part
   of the group identity by design, `DuplicateGroupDTO.repository_id`).
3. Mutations: `MergeGroup`/`DismissGroup` load the group and require
   `group.owner_id == caller || admin` (mirror `getAuthorizedAlbum`;
   NULL-owner groups are admin-only). Return 404 on foreign groups to avoid
   existence leaks, matching asset authz behavior.
4. Reads: `ListDuplicateGroups`/`GetDuplicateSummary`/`GetDuplicateGroup`
   scope by `ownerScopeID(c)`.

### A2. Face clusters / people

1. Schema: add `owner_id integer REFERENCES users(id) ON DELETE CASCADE`
   (nullable) to the `face_clusters` CREATE TABLE in migration 000005.
   Nullable because `assets.owner_id` is nullable — a cluster over
   ownerless assets has a NULL owner and stays admin-only.
2. Clustering scope: drop `RepositoryID` from `faceClusterScope` so the
   scope becomes `{OwnerID, EmbeddingModel}` — automatic clusters may span
   repositories (the product intent) but never owners. `RebuildFaceClusters`
   keeps its optional repository parameter as a *face selection* filter
   (which faces get re-assigned), not as a cluster-identity partition:
   a repo-B face may join a cluster whose members live in repo A, same owner.
   Stamp `owner_id` when a cluster is created.
3. Authorization: replace the ANY-match `EXISTS` pre-flight with
   `cluster.owner_id == caller || admin`, and **drop `repositoryID` from the
   mutation pre-flight** (`GetPerson` inside merge/move/remove/cover/hidden
   paths, `people_handler.go:558+`) — a person is not repository-scoped, so
   that filter can 404 a person that legitimately exists. Keep repository as
   a read-time display filter on person asset grids only.
4. Corrections: `MergePeople`/`MoveFace` reject cross-owner targets
   (`source.owner_id == target.owner_id` or admin). Merge candidates
   (`GetClusterMergeCandidates`) add a mandatory same-owner join condition.
5. `is_hidden` stops being a cross-owner problem by construction once each
   cluster has one owner; no per-user hide table needed.

### A3. Asset stacks

1. `FindCandidatesForStackingByName`: add owner to the candidate filter and
   to the grouping key so auto-detected stacks never span owners (same
   treatment manual create/unstack/live-photo already have).
2. `GetAssetStack`: filter the returned member list to assets the caller may
   see (owner-or-admin), consistent with the browse queries. Optionally add
   `asset_stacks.owner_id` to the 000004 CREATE TABLE for symmetry, but the
   member filter is the required fix.

### A4. Schema consistency

- `location_clusters.owner_id`: change the 000004 CREATE TABLE from
  `NOT NULL DEFAULT 0` to nullable with FK to `users`, keeping the UNIQUE
  constraint semantics via `UNIQUE NULLS NOT DISTINCT` or a coalesce index —
  verify the rebuild job's upsert still conflicts correctly.

### A5. Codegen and contracts

```bash
cd server && sqlc generate
make dto
```

DTO changes: `DuplicateGroupDTO`/`PersonDetailDTO` need no new public
fields (owner is enforced, not displayed); do **not** add `repository_id` to
`PersonDetailDTO`.

## Workstream B — Frontend Scope Discipline

The rule surface shrinks to: list pages use `useBrowseScope`, upload uses
`useWorkingRepository`, entity actions use the entity's own data, maintenance
jobs live in Manage.

1. **People hooks** (`usePeople.ts`): drop the `repositoryId` parameter and
   the `useBrowseScope()` fallback from every *mutation* hook
   (`useMergePeople`, `useMoveFace`, `useRemoveFaceFromPerson`,
   `useSetPersonCover`, `useSetPersonHidden`, rename in `usePersonDetails`)
   — people are not repository-scoped. List hooks (`usePeople`,
   `usePersonFaces` for grid display) keep browse scope for reads.
2. **PersonDetails.tsx**: remove `useBrowseScope()` entirely; the page needs
   only the person id. (It is a "list of one" — no browse filtering happens
   there.)
3. **AlbumDetails.tsx**: stop passing `scopedRepositoryId` into the album
   asset query — an album intentionally spans repositories; browse scope
   must not hide album members.
4. **Duplicates.tsx**: list/summary reads switch `useWorkingRepository` →
   `useBrowseScope`; per-group merge/dismiss use `group.repository_id`
   (entity-owned, already in the DTO); the Scan/Scan-All button moves to the
   per-repository controls in Manage (category 4) and is removed from the
   browse page.
5. **People.tsx**: remove the `useWorkingRepository` write-scope; the
   "Rebuild face clusters" action moves to Manage as an explicit per-repo
   control **defaulting to all repositories** (cross-repo identity is the
   intended output, per A2.2).
6. **Working repository converges to the upload panel — its only surface.**
   Remove the `<select>` from `NavBar.tsx` (its browsing role is already
   taken over by `BrowseScopeSelect` in page headers) and remove the
   "Working repository" group from Settings ServerTab (its description still
   sells the old dual browse/write role, which no longer exists). No Manage
   row action either — picking a target in Manage and picking it at upload
   would be the same act with two entrances. The picker lives in
   `UnifiedUploadSection` (which already resolves
   `selectedRepository ?? primaryRepository`), shown at the moment the
   choice takes effect. `useWorkingRepository`'s consumers shrink to the
   upload feature.
7. **Repository listing for non-admins**: `GET /assets/indexing/repositories`
   is `RequireAdmin()`-gated (`router.go:380`), yet `BrowseScopeSelect` and
   the working-repository hooks depend on it — non-admin users currently see
   "Repository options unavailable" on every list page. Add a non-admin
   read of the shared registry exposing only `id` + `name` (never `path`),
   either as a new lightweight endpoint or by relaxing this one with
   role-based field trimming.
8. **Cleanup**: delete the dead `repository_id` field from
   `filters.slice.ts` if still unused after the above.
9. **i18n**: new/moved strings via `t("key", "default")` then
   `vp exec i18next-cli extract`; fill zh values.

## Workstream C — Documentation

1. Write the model down permanently in
   `site/docs/internal/agent/scoping.md`: the two axes, the two ownership
   rules, the four-sources table, and the "structurally single-repo" entity
   table. Link it from `BACKEND.md` (authorization section) and
   `FRONTEND.md` (state boundaries section).
2. Update `web/src/features/collections/doc.ts` where pages change
   (Duplicates, People); regenerate `doc.md`.
3. Move `folders-tags.md` to `exec-plans/completed/` once the baseline diff
   lands; move this plan when done.

## Defaults Chosen (flag if you disagree)

- **Migrations are edited in place, no upgrade path** (user decision,
  2026-07-01): pre-production, dev environments are reset for validation.
  No backfill or startup self-heal is needed — owner stamps come from
  clustering/detection running fresh on a reset database.
- **Browse scope stays a persisted preference** (`browseRepositoryId`),
  not per-session, despite the original "ephemeral" framing: it is
  read-only and never feeds a mutation, so persistence carries no
  correctness risk, and it keeps the current implementation unchanged.

## Sequencing

1. Land the current uncommitted Folders/Tags + browse-scope diff (baseline).
2. A1 (duplicates) — the live authorization gap, independently shippable.
3. A2–A4 + in-place edits to migrations 000004/000005 as one backend
   change; then A5 codegen.
4. B1–B9 frontend, which depends on A2.3 (dropping the repository pre-flight)
   so person mutations without a repository filter succeed; B7 (non-admin
   repository listing) is backend-touching and can ride along with A5.
5. C documentation.

## Validation

```bash
cd server && sqlc generate
make dto
make server-test
make web-test
```

Manual smoke (two-user instance):

- User B cannot merge/dismiss/see user A's duplicate groups; admin sees all.
- Rebuilding clusters yields per-owner people; a person spanning two
  repositories appears as one person; user B cannot rename/merge/hide user
  A's person; merge suggestions never pair different owners.
- Person rename/merge works with browse scope set to a repository that
  contains none of that person's faces (the old 404 case).
- `GET /assets/:id/stack` never lists another owner's asset IDs.
- Album detail shows all members regardless of browse scope.
- Upload target always shows one concrete repository; Duplicates/People
  browse pages no longer host scan/rebuild buttons; Manage does.
- NavBar and Settings have no repository selector; the upload panel is the
  only place to pick/see the upload target; a non-admin user sees real
  repository names (not paths) in `BrowseScopeSelect` instead of
  "options unavailable".

## Risks

- **Dropping repository from cluster scope** changes incremental clustering
  behavior (new faces can now join clusters in other repos). This is the
  product intent, but rebuild results will differ; the per-repo rebuild
  option in Manage stays for cost control.
- **`UNIQUE` semantics with nullable `location_clusters.owner_id`**: NULL
  breaks naive UNIQUE conflict targets; verify the geo rebuild upsert path.

## Critical Files for Implementation

- `server/internal/db/repo/queries/duplicates.sql`
- `server/internal/service/face_clustering.go`
- `server/internal/api/handler/duplicate_handler.go`
- `server/migrations/000004_collections_locations_duplicates.up.sql`
- `web/src/features/people/hooks/usePeople.ts`

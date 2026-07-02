# Ownership And Repository Scoping

Lumilio has exactly two scoping axes. They are orthogonal, and every piece of
scope-aware code should be explainable in terms of this page. If a change
doesn't fit, the change is probably wrong — or this page needs a deliberate
update, not a silent exception.

## The two axes

- **Repository** (`repository_id`) — *where media physically lives*.
  PhotoPrism-style shared storage registry: repositories are admin-managed
  locations inside one unified library, carry **no per-user ownership**, and
  are union-friendly. A person, album, or tag spanning repositories is
  correct behavior, not a bug. `repositories.default_owner_id` is a fallback
  owner for newly discovered files, not an ACL.
- **Ownership** (`assets.owner_id`) — *whose media it is*. Immich-style hard
  partition and the **only** visibility/mutation boundary. Admins see
  everything; regular users see exactly their own. There is no sharing
  system; cross-user viewing is admin-only.

## The two ownership rules

1. **Grouping entities include the owner in their grouping key —
   structurally, not as an optional filter.** Face clusters, duplicate
   groups, asset stacks, and location clusters each carry an `owner_id`
   column, and detection/clustering never produces a group that spans
   owners. NULL-owner groups (built from ownerless assets) are admin-only.
2. **Mutations on a grouped entity require whole-entity ownership or admin —
   never "owns ≥ 1 member".** Because of rule 1 this is a plain equality
   check on the entity's `owner_id`. Foreign entities return 404, not 403,
   so their existence is not leaked (`duplicateGroupOwnedBy`,
   `faceService.authorizePerson`).

Corrections must preserve rule 1: merging people or moving a face across
owners is rejected (`ErrPeopleCrossOwner`) even for admins, because the
result would be a mixed-owner cluster.

## The four sources of "which repository"

| # | Source | Lifetime | Used for |
|---|--------|----------|----------|
| 1 | Browse scope (`useBrowseScope`) | Sticky preference, read-only | "What am I looking at?" — list pages only ("All" is a valid default). Never feeds a mutation. |
| 2 | Working repository (`useWorkingRepository`) | Sticky, always resolves to one concrete repo (primary fallback) | "Where does new content land?" — upload only. Its single UI surface is the picker in the upload panel. |
| 3 | Entity-owned `repository_id` | Derived from data | Actions on one loaded, structurally single-repo entity: asset, folder, duplicate group. |
| 4 | Explicit per-repo action target | One-shot, chosen by the clicked row | Maintenance jobs in Manage: rescan, stack detection, duplicate scan, location rebuild. |

Entities that are **not** structurally single-repo — albums and people — take
no repository filter on their detail pages or mutations. Repository may still
appear as a *display filter* on their member lists (e.g. the people grid
under a browse scope), but never in authorization or a pre-flight check: a
person must not 404 just because the browse scope points at a repository
without their faces.

## Backend reference points

- `ownerScopeID(c)` (`internal/api/handler/asset_authz.go`) — returns nil for
  admins, the user ID otherwise. Every owner-scoped read passes it; every
  grouped-entity mutation passes it as the required owner.
- Face clustering partitions by `faceClusterScope{OwnerID, EmbeddingModel}`
  (`internal/service/face_clustering.go`) — repository is deliberately *not*
  part of cluster identity; it survives only as a face-*selection* filter in
  rebuilds (which faces get re-assigned, not which cluster they may join).
- Duplicate detection groups by owner inside a repository
  (`internal/service/duplicate_service.go`); a duplicate group is
  structurally single-repo *and* single-owner.
- Maintenance endpoints (`/duplicates/detect`, `/locations/rebuild`,
  indexing stats/rebuild) are admin-only; the repository registry read
  (`GET /assets/indexing/repositories`) is open to all authenticated users
  but strips filesystem paths for non-admins.

## Frontend reference points

- `useBrowseScope` (`web/src/features/settings/hooks/useBrowseScope.ts`) —
  list pages (Home, Folders, Tags, Albums, Collections, People, Map,
  Duplicates) via `BrowseScopeSelect` in page headers.
- `useWorkingRepository` — consumed only by the upload feature
  (`UnifiedUploadSection` renders the picker; `useUploadProcess` reads the
  resolved target).
- People mutations (`usePeople.ts`) take no repository arguments; the person
  edit modal (`PersonRenameModal`/`PersonFacesPanel`/`PersonPicker`) is
  entirely unscoped.
- Maintenance actions live in Manage (`RepositoryGrid`): per-repository scan
  / stack detection / duplicate scan / location rebuild, plus the
  library-wide people rebuild.

Linked from [BACKEND.md](./BACKEND.md) and [FRONTEND.md](./FRONTEND.md).
Decision history: `exec-plans/completed/ownership-scope.md`.

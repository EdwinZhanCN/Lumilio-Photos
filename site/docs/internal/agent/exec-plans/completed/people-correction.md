# People Correction

> **Status: Completed (2026-06-27).** Implemented merge, move-face, remove-face,
> set-cover, and hide/unhide end to end. Backend: migration `000007` adds
> `face_clusters.is_hidden`/`hidden_at` (+ index); new SQL in `faces.sql`/
> `people.sql`; `FaceService` correction methods with owner/repository
> authorization via `GetPersonByIDScoped`; manual corrections marked
> `is_manual` and **replayed across full rebuild** so they survive. New routes
> under `/api/v1/people/{id}/...` plus a per-face crop endpoint
> (`/faces/{faceId}/crop`). Frontend: hooks in `usePeople.ts`, a visible/hidden
> toggle on the people grid, a correction action bar + `PersonFacesPanel` on
> `PersonDetails`, and `PersonMergeModal`/`MoveFaceModal`/`RemoveFaceModal`
> sharing a `PersonPicker`. `make dto`, `make web-test`, and `make server-test`
> all pass.
>
> **Caveat:** rebuild-preservation is exercised by the live code path but has no
> DB-backed integration test — the face suite is pure-unit and no DB harness
> exists in `internal/service`. Open questions (exclusion markers, auto-confirm
> on move, hidden-people in agent facets) were left as-is; merge accepts
> multiple sources.

## Goal

Make recognized people correctable by the user.

The first people milestone already lists, rebuilds, renames, and opens a
person-scoped gallery. P1 closes the practical face-recognition loop: when the
model groups faces incorrectly, the user must be able to fix it without waiting
for a full rebuild or editing database state.

Target capabilities:

- merge duplicate people
- move a wrongly assigned face to another person
- remove a face from a person
- set a person's cover/representative face
- hide and unhide people from the main people grid

## Product Principles

- Keep AI assistive. The user remains the authority for identity corrections.
- Corrections must survive incremental face recognition and full cluster rebuilds
  where possible.
- Every correction must be owner/repository scoped.
- Do not expose demographic face attributes or raw embeddings in the UI.
- Favor review-style workflows over hidden one-click destructive actions.

## Non-Goals

- No automatic name suggestions.
- No cross-user global identity graph.
- No face editing/crop adjustment.
- No manual drawing of face bounding boxes.
- No comments, memories, or social features.
- No public sharing changes in this plan.

## Current State

Backend:

- `PeopleHandler` exposes list, rebuild, get, rename, cover, and person asset
  list endpoints.
- `FaceService` exposes `ListPeople`, `GetPerson`, `RenamePerson`, and rebuild
  operations.
- `face_clusters` already has `representative_face_id`, `cluster_name`,
  `is_confirmed`, and `member_count`.
- `face_cluster_members` already has one exclusive assignment per face through
  `face_cluster_members_face_unique_idx`.
- SQL already contains useful primitives:
  - `AssignFaceClusterMemberExclusive`
  - `DeleteFaceClusterMember`
  - `CopyFaceClusterMembersToCluster`
  - `MergeFaceClusters`
  - `DeleteEmptyFaceClusters`
  - `UpdateFaceClusterRepresentative`

Frontend:

- `/collections/people` renders `PeopleCollectionGrid`.
- `/people/:personId` renders a person hero, rename modal, stats, and a
  person-scoped `AssetsGalleryPage`.
- `usePeople` and `usePersonDetails` wrap the generated people API.

## Data Model

### Hide People

Add columns to `face_clusters`:

```sql
ALTER TABLE face_clusters
  ADD COLUMN is_hidden boolean NOT NULL DEFAULT false,
  ADD COLUMN hidden_at timestamptz;
```

Add an index:

```sql
CREATE INDEX face_clusters_hidden_idx
  ON face_clusters (is_hidden, updated_at DESC);
```

`is_hidden` hides a person from default people views. It does not remove faces,
assets, names, or cluster assignments.

### Manual Correction Durability

Use existing `face_cluster_members.is_manual` as the durable marker for manual
assignment. Manual assignments should not be overwritten by incremental
recognition. Full rebuild behavior needs an explicit decision:

- preserve manual assignments by replaying them after rebuild, or
- document that rebuild may discard manual membership corrections.

The recommended implementation is to preserve manual assignments. If the
current rebuild path deletes all membership in scope, capture manual
assignments before deletion and reapply them after automatic clustering, then
refresh representatives and delete empty clusters.

## Backend Plan

### 1. Extend DTOs

Update `server/internal/api/dto/people_dto.go`.

Add fields:

- `is_hidden` to `PersonSummaryDTO`
- `hidden_at` to `PersonSummaryDTO`

Add request/response DTOs:

- `MergePeopleRequestDTO`
- `MoveFaceRequestDTO`
- `RemoveFaceRequestDTO`
- `SetPersonCoverRequestDTO`
- `SetPersonHiddenRequestDTO`
- `PersonFaceDTO`
- `ListPersonFacesResponseDTO`

`PersonFaceDTO` should include only UI-safe fields:

- face item ID
- asset ID
- face crop URL or image path indicator
- confidence
- is representative
- is manual
- asset filename when useful
- taken/upload time when useful

Do not expose embeddings, repository paths, raw bounding JSON, pose angles, or
demographic attributes.

### 2. Add SQL Queries

Extend `server/internal/db/repo/queries/faces.sql`.

Required queries:

- list face items for a person with asset scope
- get a face item with asset scope
- set cluster hidden state
- set representative face ID
- assign face to cluster exclusively as manual
- remove face from cluster
- merge cluster members into target cluster
- delete empty clusters
- list potential merge targets/search people by name
- count visible/hidden people separately

Prefer queries that join `assets` for repository/owner authorization rather than
trusting a bare cluster ID.

Run:

```bash
cd server && sqlc generate
```

### 3. Extend Face Service

Add service methods to `FaceService`:

```go
ListPersonFaces(ctx, clusterID, repositoryID, ownerID, pagination)
MergePeople(ctx, targetClusterID, sourceClusterIDs, repositoryID, ownerID)
MoveFace(ctx, faceID, targetClusterID, repositoryID, ownerID)
RemoveFaceFromPerson(ctx, faceID, clusterID, repositoryID, ownerID)
SetPersonCover(ctx, clusterID, faceID, repositoryID, ownerID)
SetPersonHidden(ctx, clusterID, hidden, repositoryID, ownerID)
```

Implementation rules:

- Load/authorize the target person before mutating.
- For merge, authorize every source person in the same repository/owner scope.
- Preserve the target name, confirmation state, and hidden state.
- Move source members into the target with `is_manual = true`.
- After merge/move/remove, refresh representatives for affected clusters.
- Delete empty unconfirmed clusters. If an empty source cluster was named or
  confirmed, delete it only after explicit merge; for a single face removal,
  prefer leaving the empty deletion behavior consistent with existing
  `refreshClusterRepresentative`.
- `SetPersonCover` must verify the face belongs to the person.
- `MoveFace` must verify the target person exists and the face is in the same
  repository/owner scope.
- `RemoveFaceFromPerson` should leave the face unclustered, making it available
  for later correction/rebuild.

Wrap multi-step corrections in transactions through the existing `withTx`
helper.

### 4. Add People Handler Routes

Extend `PeopleControllerInterface` and `PeopleHandler`.

Authenticated endpoints:

```http
GET   /api/v1/people/{id}/faces
POST  /api/v1/people/{id}/merge
POST  /api/v1/people/{id}/faces/{faceId}/move
POST  /api/v1/people/{id}/faces/{faceId}/remove
PUT   /api/v1/people/{id}/cover
PUT   /api/v1/people/{id}/hidden
```

Query parameters:

- `repository_id` should match existing people routes.
- `include_hidden` belongs on `GET /api/v1/people`.

Handler rules:

- All correction routes require auth.
- Admins can operate across owners only when existing ownership helpers allow it.
- Non-admins are owner-scoped.
- Return updated `PersonDetailDTO` or a focused correction response with affected
  person IDs/counts.

OpenAPI annotations must reference concrete DTOs.

### 5. Rebuild Preservation

Inspect `RebuildFaceClusters`.

If rebuild deletes all memberships in scope:

1. Load manual memberships in scope before deletion.
2. Run automatic clustering.
3. Reapply manual memberships with exclusive assignment.
4. Refresh affected representatives.
5. Delete empty clusters.

Add tests so manual corrections survive rebuild.

## Frontend Plan

### 1. People Hooks

Extend `web/src/features/people/hooks/usePeople.ts`.

Add hooks:

- `usePersonFaces`
- `useMergePeople`
- `useMoveFace`
- `useRemoveFaceFromPerson`
- `useSetPersonCover`
- `useSetPersonHidden`

Invalidate:

- `["get", "/api/v1/people"]`
- `["get", "/api/v1/people/{id}"]`
- person asset list queries
- asset list/search queries when person filters can be affected

Do not cast around generated API types.

### 2. People Grid

Update `/collections/people`.

Controls:

- segmented control or toggle: visible / hidden
- rebuild action remains in the page header
- hidden people show a hidden badge in hidden mode

Default grid excludes hidden people.

### 3. Person Detail Correction Panel

Add a correction panel to `PersonDetails`.

Recommended layout:

- keep current `CollectionHero`
- add a compact action cluster in the hero or a right-side panel:
  - Rename
  - Merge
  - Hide/Unhide
  - Set cover from selected face
- add a `Faces` tab or panel below the hero that lists face crops for the person

Face crop grid actions:

- set as cover
- move to another person
- remove from this person

The main asset gallery remains below or beside the correction panel. Avoid
burying correction actions inside the full-screen asset carousel only.

### 4. Merge Flow

`PersonMergeModal`:

- search/select one or more source people
- show target person at the top
- show selected source people with covers and counts
- confirmation copy: source people will be merged into target; assets remain in
  the library; this can change people filters
- after success, navigate/remain on target person and invalidate lists

Do not offer merge from hidden people by default unless the hidden toggle is on
or a search explicitly returns hidden people.

### 5. Move Face Flow

`MoveFaceModal`:

- launched from one face crop
- search/select target person
- optional create-new-person path can be deferred unless backend supports it
- after success, remove the face from the current face grid and refresh stats

### 6. Remove Face Flow

Use a confirmation modal:

- copy should say the face will no longer be associated with this person
- original asset remains unchanged
- face can be found again by rebuild/correction later

### 7. Set Cover Flow

Fast action from face crop:

- no modal needed, but show a success/error toast
- optimistic update is optional; invalidating person detail is enough

### 8. Feature Docs

Add/update `web/src/features/people/doc.ts` to describe:

- person list/detail ownership
- correction operations
- distinction between asset gallery and face-level corrections

Generate `doc.md`.

## UX Details

- Use face crops for correction grids, not full asset thumbnails.
- Keep destructive-looking actions text-first with icon support.
- Hide is not delete. Use `EyeOff`, not `Trash2`.
- Moving/removing a face should not imply the original media is modified.
- Show empty states for:
  - no detected faces
  - hidden people list empty
  - merge search no results
  - face crop missing on disk

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

- rename still works
- hide person removes them from default people grid
- hidden mode shows hidden people
- unhide restores them
- set cover changes people grid and person hero cover
- merge two people and verify source disappears/empties and target asset count
  increases
- move one face to another person and verify both person counts update
- remove one face and verify it leaves the current person
- full person asset gallery still filters by the updated cluster membership
- rebuild preserves manual move/merge corrections, or the documented behavior is
  enforced by tests

## Risks And Decisions

- **Manual correction durability**: preserving corrections through rebuild is
  important but touches clustering internals. If this is too large, ship
  correction APIs first and explicitly warn that full rebuild can discard manual
  corrections. Preferred path is preservation.
- **Cluster identity after merge**: target cluster ID should survive. Source
  cluster IDs may be deleted after merge; frontend should navigate to target.
- **Empty confirmed clusters**: deleting an empty named person can surprise the
  user. Merge can delete source clusters because the user explicitly requested
  it. Remove-face should avoid surprising deletion copy.
- **Representative face validity**: cover must be a face in the target cluster,
  not just any face crop.
- **Authorization**: cluster IDs alone are not enough. Every mutation must join
  through face item -> asset to enforce repository and owner scope.
- **DTO drift**: people APIs currently have concrete DTOs. Keep that discipline;
  run `make dto` and do not cast on the frontend.

## Open Questions

- Should `Remove face` create a "Not this person" exclusion so incremental
  recognition cannot immediately reassign it to the same cluster?
- Should moving a face to a target person mark both target and source as
  confirmed?
- Should merge allow multiple source people in one request, or start with a
  single source person for simpler UI?
- Should hidden people be excluded from Lumilio agent facets and autocomplete?

## Critical Files for Implementation

- `server/internal/service/face_service.go`
- `server/internal/service/face_clustering.go`
- `server/internal/db/repo/queries/faces.sql`
- `server/internal/api/handler/people_handler.go`
- `web/src/features/people/routes/PersonDetails.tsx`

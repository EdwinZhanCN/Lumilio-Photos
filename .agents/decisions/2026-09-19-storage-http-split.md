# Decision: Split storage HTTP into targets, view, and POST detach

Status: implemented

## Problem

Ordinary uploaders and administrators shared overlapping list endpoints
(`/assets/indexing/repositories`, `/storage-locations`) that leaked paths,
Location IDs, capacity, and verification, while Manage fetched admin Location
and native-capability data. Unregistration used body-dependent DELETE.
Setup created the Primary Repository through the same create route as later
regular Repositories. Compatibility aliases would have frozen that split.

## Decision

Authenticated non-admins read `GET /api/v1/storage/targets`:
`{ targets: [{ id, name, role, read, upload }] }` with closed admission
reasons (`offline`, `identity_error`, `read_only`, `low_space`, `paused`,
`busy`, `recovery_required`) and empty `reasons` as `[]`. The payload has no
path, Storage Location, capacity, or verification.

Administrators read `GET /api/v1/storage/view` (`locations`, `repositories`,
`capacity_groups`, `observed_at`) and command `/api/v1/storage/*`.
Unregistration is `POST .../detach-impact` and `POST .../detach`. Verification
is `/storage/repositories/{id}/verifications`. Native grants use
`/storage/native-capability` and `/storage/native-tasks`; approval paths and
nonces stay in the Desktop control plane. Standalone/Docker open existing
Repositories through `/storage/candidates` by portable directory name.

Initial primary creation is `POST /api/v1/setup/primary-repository`, outside
completed-setup gates. `POST /storage/repositories` rejects `role=primary`.
Cloud and stack commands remain on `/repositories/{id}/cloud*` and
`/stacks/detect`.

Manage (`/manage`) consumes only targets. Admin Storage is `/storage`. There
are no alias routes, deprecated fields, or dual-served shapes.
`web/src/lib/http-commons/schema.d.ts` is generated from OpenAPI.

## Alternatives considered

**Keep `/assets/indexing/repositories` and add a second admin view.** Rejected
because two selector DTOs (`is_primary` plus `role`, admin `path`, `storage_location_id`)
forced frontend guesses about eligibility and made Manage request admin data.

**Alias old routes onto the new handlers during cutover.** Rejected because
this application has no deployed instance to migrate; aliases would freeze the
wrong authority split.

**Unregister with DELETE and a JSON body.** Rejected because body-dependent
DELETE is easy to send wrong, and detach is an explicit catalog command with
a disclosed impact, not resource deletion of originals.

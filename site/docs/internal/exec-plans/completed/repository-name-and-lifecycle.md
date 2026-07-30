# Repository Name And Lifecycle

## Goal

Make repository creation predictable for Server bind mounts: a regular
repository's validated name is its exact directory name, unsupported characters
fail before filesystem mutation, and initialization proves the selected target
is writable without deleting a pre-existing mount on rollback. Keep repository
update/delete implementation available for future work, but stop exposing those
operations through the shared HTTP API.

## Contracts

- Regular repository names preserve case and are not slugified.
- Allowed characters are Unicode letters, Unicode decimal digits, ASCII spaces,
  ASCII hyphens, and underscores.
- Names are 1–80 Unicode code points, at most 240 UTF-8 bytes, and cannot start
  or end with a space.
- The primary repository remains the fixed `<storage.path>/primary` bootstrap
  directory.
- Repository creation distinguishes invalid names, path conflicts, and
  filesystem write/init failures.
- A pre-existing empty Server bind mount is a valid initialization target.
- Rollback removes only artifacts created by the failed initialization and
  never recursively removes a pre-existing target directory.
- `PATCH /api/v1/repositories/:id` and
  `DELETE /api/v1/repositories/:id` remain in source but their route
  registrations are commented out.

## Final Contracts

- Regular repository names preserve case and become the exact directory name;
  the primary repository remains `<storage.path>/primary`.
- The shared Go and Web rule accepts Unicode letters/digits, ASCII spaces,
  hyphens, and underscores, with 80-code-point and 240-byte limits.
- Creation rejects case-insensitive sibling conflicts, non-empty targets, and
  unwritable storage before inserting the repository row.
- Initialization rollback preserves pre-existing bind-mount targets and removes
  only Lumilio-created artifacts from them.
- Repository PATCH and DELETE handlers remain in source, while their runtime
  route registrations and OpenAPI route annotations are disabled.
- Chinese documentation describes Desktop's multiple Storage Locations,
  Server's single `/data/storage` root with multiple child mounts, and the
  mount-first/create-second workflow.

## Validation

- `make server-test`
- `make dto`
- `make web-test`
- `vp exec i18next-cli status` (Chinese 100%)
- `vp run build` in `site/`
- `git diff --check`
- Generated OpenAPI exposes only GET for `/api/v1/repositories/{id}`.

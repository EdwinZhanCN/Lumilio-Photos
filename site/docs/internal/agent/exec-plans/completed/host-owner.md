# Host Owner

## Goal

Make the initial administrator the single Host Owner for repository fallback
ingestion without introducing per-repository ACLs or user groups.

## Work

- Resolve one stable Host Owner from the primary repository, falling back to
  the first account during bootstrap.
- Use it for Web repository creation and Desktop attach/copy operations.
- Require initial-administrator setup before Desktop can attach a repository;
  Storage Locations themselves may still be authorized earlier.
- Preserve explicit upload and cloud-import owners.
- Remove `default_owner_id` from the mutable repository API.
- Migrate existing repository defaults and ownerless assets, including their
  structural owner-bearing records.
- Cover the control-plane and API boundaries with focused tests and regenerate
  SQL/OpenAPI artifacts.

## Validation

- `make server-test`
- `make desktop-test`
- `make web-test`
- Applied the migration to an isolated PostgreSQL database containing mixed
  ownerless and explicitly owned records.

## Status

Completed on 2026-07-21.

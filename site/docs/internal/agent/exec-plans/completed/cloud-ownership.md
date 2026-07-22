# Cloud Ownership

## Goal

Separate shared repository fallback ownership from user-owned cloud accounts
and keep cloud-imported asset ownership stable across manual and automatic
sync runs.

## Work

- Treat `cloud_credentials.owner_id` as the cloud account's authorization
  boundary.
- Let regular users list and manage only their own credentials; administrators
  retain global access.
- Store the credential owner on each repository cloud binding.
- Snapshot the binding owner on each import run and use it for materialization.
- Expose Cloud settings to regular authenticated users.
- Migrate legacy credentials, bindings, and runs without changing explicit
  asset owners.

## Validation

- `make server-test`
- `make web-test`
- `make desktop-test`
- Applied migration `000011` to an isolated PostgreSQL database with ownerless
  and explicitly owned credentials; credentials, bindings, and runs resolved to
  the expected `1,2` owner sequence.
- `git diff --check`

## Status

Completed on 2026-07-21.

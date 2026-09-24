# Decision: Replace the catalog migration chain with a generation-9 baseline

Status: superseded on 2026-09-24 by
[the rc.1 compatibility baseline decision](2026-09-24-rc-compatibility-baseline.md).
Originally implemented 2026-09-19. Its rejection of a migration chain rested on
"no instance exists to upgrade", which stops being true at the rc.1 tag.

## Problem

The SQLite catalog carried a sequenced, checksummed migration ledger so that
existing databases could upgrade in place. This rewrite is allowed to discard
application state, but backups still need a real schema discriminator: an
absent ledger must not look like a compatible empty catalog.

## Decision

One standalone baseline, `server/migrations/000001_storage_baseline.up.sql`,
creates the complete STRICT schema. `PRAGMA user_version = 9` is the only
schema discriminator. `InspectStandaloneCatalog` and backup manifests compare
`schema_version` from that pragma; version 0 is incompatible. There is no
historical migration file, generation counter, or checksum ledger for catalog
history. QueueDB River migrations remain independent and disposable.

User media files are never modified or deleted by this cutover. After the
rewrite, ordinary rename, reconnect, and temporary unavailability still
preserve user-authored catalog metadata; explicit detach has separately
disclosed catalog effects.

## Alternatives considered

**Keep the migration chain and add additive steps for the new storage
model.** Rejected because no instance exists to upgrade, and additive
migrations would preserve the dual authorities this rewrite removes.

**Drop every version check and treat any SQLite file as a fresh catalog.**
Rejected because backups must fail closed when the schema is missing or
wrong; an absent ledger yielding zero is not compatibility.

**Infer compatibility from application tables existing without
`user_version`.** Rejected because that cannot distinguish a truncated file
from a current catalog and reintroduces ledger-shaped heuristics.

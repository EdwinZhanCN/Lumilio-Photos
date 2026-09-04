# Decision: Wake OCR index drains from committed mutations

Status: implemented

## Problem

OCR search correctly treats SQLite rows and its revision outbox as
authoritative, but the queue inserted and executed a River outbox-drain job
every second even when the outbox was empty. A continuously running idle
instance therefore produced roughly 86,400 no-op jobs per day, added SQLite
queue churn, and made Monitor look as if the Bleve index were constantly being
rebuilt. The planned transcript sidecar was also positioned to copy this
scheduling pattern.

## Decision

Committed OCR result changes and asset Trash/restore changes send a
best-effort process-local notification to the OCR outbox trigger. Notifications
within a tick coalesce into one pending wake. The one-second River periodic
constructor remains only as an in-memory debounce check: it returns no job
while idle and inserts a drain job after a notification.

The revisioned SQLite outbox remains the durable recovery boundary. The
trigger schedules an unconditional drain at startup and once per minute so a
commit followed by a process crash, a missed notification, or a failed job
insertion cannot strand index work indefinitely. The drain job retains
one-second period uniqueness so a mutation arriving while an earlier drain is
running can create a later follower without duplicate jobs inside one tick.

## Alternatives considered

**Keep the unconditional one-second River job** — rejected because it turns an
idle consistency check into persistent queue and SQLite write load, and makes
operational activity misleading.

**Only increase the fixed polling interval** — rejected as the final design.
It reduces churn but couples normal search freshness to the recovery interval
and continues creating empty jobs forever.

**Insert one River job transactionally for every outbox mutation** — rejected
because the outbox already coalesces work and large OCR imports would create a
second per-asset job stream whose trailing jobs mostly drain nothing.

**Use only process-local notifications** — rejected because a notification is
intentionally not durable. The startup and one-minute recovery drain are the
required complement to the SQLite outbox.

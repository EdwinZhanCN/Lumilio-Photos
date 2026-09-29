# Decision: One admin storage surface with capacity as a Repository fact

Status: implemented

## Problem

Storage facts were spread over three surfaces and two read models. Manage
rendered a Repository eligibility list that repeated its own upload-target
selector; Server Monitor rendered a second full storage tree from
`/api/v1/storage/diagnostics` and attributed capacity and writability to a
Storage Location; the admin Storage page grouped cards by Location but took a
Location's capacity from the first Repository it found, so one mount's figure
stood in for every sibling.

Three consequences followed. A reader could see three different answers for one
disk. Both a Storage Location figure and a Repository figure existed, although
only the Repository path is measured. And the page structure changed shape with
the data, so no stable visual language could form.

The shared `Modal` compounded the presentation problem: it was a `div` with a
hand-written Escape listener, so it had no dialog role, no focus trap, no focus
restoration, and left the page behind it interactive.

## Decision

**`/storage` is the only admin storage surface.** It has two views behind one
tab strip — Browse and History — and a structure that does not change with the
data. The page header carries the title, count summary, `observed_at`, the
primary create action in its action slot, and overflow commands. The view tabs
are content, not chrome: they sit at the top of the scrolling content area, so
the header carries identity and actions only and never a second navigation
strip. Browse renders one table per Storage Location in the model's own order:
it is a reading surface with no search, filter, or sort.
History renders the lifecycle audit as a flat list with no row expansion,
because a recorded action has nothing to disclose.

**Capacity is a Repository row fact.** The admin read model exposes
`mount_path` and `filesystem` per Repository — raw host data, never localized —
and each row shows the figure its own path measured together with the storage
label. Capacity never appears in the page header or on a Storage Location
header, because a Location can hold Repositories on different mounts (a Docker
child mount), where a single Location figure would misattribute at least one of
them. `deriveStorageLabel` maps the mount path to the storage identity a reader
recognizes, treating a machine's own system mounts as one built-in storage.

**Row facts open in place, at one level.** Selecting a Repository expands an
inline inspector beneath its row: every fact — identity, storage, capacity,
state, contents, verification, and the diagnostic facts from
`/api/v1/storage/diagnostics` — sits in one plain-text grid, a single column on
narrow screens and two from `sm` up. There is no nested disclosure and no
second presentation of the same fact: capacity appears as one sentence that
names its storage, never as a gauge beside a repeated number. Commands live in
the row's own menu, so an action always names its Repository.

**Storage leaves the other surfaces.** Manage composes the upload editor only.
Server Monitor has no storage tab: it reports processing health, and the
lifecycle audit and support bundle moved to `/storage`.

**The shared `Modal` is a native `<dialog>`** opened with `showModal()`, which
supplies the dialog role, focus trap, focus restoration, Escape handling, and an
inert page. Its props contract is unchanged.

## Alternatives considered

**Group the page by backing capacity pool.** Rejected: when sharing is
unprovable — Docker Desktop, network mounts — every measured path becomes its
own pool, so the page would render one single-row section per Repository. The
pool axis fails exactly where it would be most useful, while grouping by Storage
Location degrades gracefully to a flat list.

**Show capacity once, wherever it is uniform (merged cells).** Rejected because
the same fact would move between the Location header and the row depending on
the values. One fact in two places is not a visual language a reader can learn,
even when each placement is individually defensible.

**Show capacity on Storage Locations only.** Rejected because it is
unrepresentable: a Location with Repositories on two mounts has two true
figures, and the alternatives there are showing both under one header (the pool
layer returning) or showing one and being wrong.

**Keep the Monitor storage tab as a read-only health overview.** Rejected: two
surfaces for one fact is what produced the disagreement, and the diagnostics
that tab rendered are reachable from the Repository row that owns them.

**Keep the Repository list on Manage.** Rejected: it repeated the upload
target selector and offered no command, so it consumed the page without
answering anything.

**Build the modal on a portal instead of `<dialog>`.** Rejected: it would
reimplement focus trapping, restoration, and inerting — browser behaviour that
already ships and that the previous implementation got wrong.

**Keep search, filtering, and sorting on this page.** Deferred, not rejected:
with a handful of Storage Locations and Repositories a reader sees everything at
once, and a sort control that only reorders a short list invites the belief that
rows are ranked. Adding any of them later must keep the row's column order and
the one-level inspector unchanged.

**Reach for the native popover API for the overflow menus.** Rejected: it is a
second floating-layer mechanism beside the one the rest of the app uses, and the
project's z-index policy already names the portal escape hatch for a floating
layer trapped by an ancestor's overflow.

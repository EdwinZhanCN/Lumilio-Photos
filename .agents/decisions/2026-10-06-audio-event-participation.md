# Decision: Events contain photos and videos; audio belongs in Music

Status: implemented — owner Edwin's 2026-10-06 decisions for issue #218,
required for v26.1.0-rc.1. Owners: `server/internal/event` and
`web/src/features/events`. Narrows the membership contract in
[Event owner-topology](2026-08-10-event-owner-topology.md).

## Problem

Audio import timestamps could seed Events or bridge photo/video segments.
Event counts included audio that the gallery did not display. Music's
designation also risked being mistaken for an Asset media type.

## Decision

At the Asset layer, `AUDIO` means only that the file is audio. Music's
`music`/`other` designation is solely a Music-projection classification:
music is `music`; recordings and voice memos are `other`. Users correct
this classification in Music. Unclassified audio continues to default to
`music`; no ingest behavior changes.

Automatic Events use only PHOTO and VIDEO logical media, including Live
Photos. Audio never seeds or influences time/location segmentation, even
with trustworthy capture metadata. Audio-only Events are never generated.
Users cannot manually add audio to an Event in rc.1.

Event rebuilds discard audio memberships, audio cover overrides, and any
correction constraint with an audio endpoint. Corrections on photo/video
members are preserved. Resolution excludes audio before computing membership,
counts, covers, repository projections, browsing/navigation, and Event share
snapshots. An audio-only stale Event is not resolvable and retires on rebuild.

No data migration is needed: pre-release catalogs are not upgraded under
[the rc.1 compatibility decision](2026-09-24-rc-compatibility-baseline.md).
Rebuild cleanup is part of projection publication, not a catalog upgrade.

General Assets gallery, Event gallery, Event Add media picker, and
collections default to photos and videos. Audio browsing belongs in Music.
The viewer and share still support audio opened or shared directly.
Event counts, covers, and viewer previous/next use the same photo/video
membership set, with Repository Browse Scope applied as a read projection.
Events need no special audio card, cover, or count behavior.

## Alternatives considered

**Include recordings with trustworthy capture time.** Rejected: the owner
chose a photo/video Event contract independent of Music classification or
audio metadata quality.

**Allow manual audio membership in rc.1.** Deferred as a possible future
feature. It would require a new presentation contract for cards, counts,
covers, picking, and navigation; none is needed for rc.1.

**Hide audio only in the gallery.** Rejected: segmentation, counts, covers,
navigation, and shares would still disagree with visible members.

**Keep audio correction constraints after filtering candidates.** Rejected:
their missing endpoints cause stale-constraint failures or retain ineligible
membership. Constraints on eligible photo/video endpoints remain intact.

**Migrate pre-release catalogs.** Rejected by the compatibility baseline;
pre-release data is disposable and is not an upgrade source.

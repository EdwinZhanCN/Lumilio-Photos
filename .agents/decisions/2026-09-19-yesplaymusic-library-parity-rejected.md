# Decision: Reject YesPlayMusic Library 1:1 parity as a completion contract

Status: rejected — 1:1 YesPlayMusic Library parity is not a remaining product
contract. Local Music remains the first-class projection in
[2026-09-06-music-library.md](2026-09-06-music-library.md).

## Problem

An active plan treated the signed-in YesPlayMusic Library as a completion
contract: matching geometry, menus, CRUD, tab scroll, lyric excerpts, artist
releases, and detail-menu parity, with remaining deviations named rather than
closed. That framing keeps a third-party listening app as the exit condition
for Lumilio Music and invites reopening source-only mutations against a
reference account.

## Decision

Do not keep or resume a YesPlayMusic 1:1 Library parity plan. YesPlayMusic may
inform layout, as the Music projection decision already records. It is not a
checklist Lumilio must finish against, and reference-account writes are not a
validation gate. Unshipped extras named by that plan — custom playlist
artwork, LRC file import, rating filters, playback history, smart playlists,
online categories, and public/private playlist controls — are ordinary future
work, not an open execution plan.

## Alternatives considered

**Finish the remaining parity items, then delete the plan** — rejected. The
leftover surface (tab scroll, likes-card lyric excerpt, artist releases,
detail menus, full-stack E2E against the reference account) is reference
imitation, not a Lumilio Music correctness gap.

**Keep the plan as a visual backlog** — rejected. Active plans are unfinished
implementation contracts. A third-party 1:1 list would drift and reopen
account-specific mutation work the plan itself marked out of scope.

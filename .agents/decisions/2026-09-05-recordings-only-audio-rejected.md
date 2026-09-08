# Decision: Reject recordings-only AUDIO scope

Status: rejected — the user requests a complete local music library.

Superseded by the implemented [Music projection and playback boundary](2026-09-06-music-library.md).

## Problem

The earlier Sound Memories plan limited AUDIO to a recordings destination,
deferred persistent playlists and treated music albums/artists as metadata
only. That restriction conflicts with the user's clarified product goal.

## Decision

The recordings-only restriction is rejected. The old execution plan is removed;
the active [Music Library plan](../../docs/exec-plans/active/music-library.md)
owns the proposed replacement. Music implementation is not yet shipped.
YesPlayMusic supplies layout reference only; DaisyUI and existing Lumilio
components govern the UI. Transcription and Agent work are not prerequisites.

## Alternatives considered

- Keep a recordings list with a transient queue: insufficient for browsing
  releases/artists and saving ordered playlists.
- Treat music albums as existing mixed-media Albums: conflates release
  metadata with user-curated asset membership.
- Reuse existing album membership for playlists: its album/asset primary key
  cannot represent repeated occurrences with independent entry identity.
- Create separate music storage/import infrastructure: unnecessary; existing
  AUDIO Assets, Locations and processing remain the file ownership boundary.

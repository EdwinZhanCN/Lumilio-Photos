# Music Library reference parity

Status: active. Reference audit and local capability mapping completed on
2026-09-07; implementation and browser validation in progress. Source-only
mutations below remain explicitly unverified against the reference account.

Goal: reproduce the observed YesPlayMusic Library composition and working
local listening interactions in Lumilio, using the existing Music domain.

## Working brief

Use the user's already signed-in YesPlayMusic browser session as the primary
reference. First operate every Library category and its reachable details,
menus, dialogs, and playback controls. Record layout, typography, spacing,
colors, hover/focus/active states, scroll behavior, navigation, and the full
create/read/update/delete interaction inventory. Cross-check each behavior
against the matching reference source and distinguish observed behavior,
source-only behavior, unsupported actions, and upstream defects.

Before implementation, map every reference operation to Lumilio's actual local
Music API and state owners. Preserve the established local-only scope. Do not
invent online data, duplicate storage concepts, copy account-specific content,
or add decorative controls without working destinations. Implement only after
this inventory is complete. Validate the resulting pages and operations in the
browser at matching viewport and theme, and run the narrow relevant repository
gates. Report any remaining discrepancy explicitly rather than calling an
approximation a 1:1 result.

## Non-goals

- Integrating NetEase accounts, recommendations, online MV catalogs, cloud
  upload, social following, public playlists, or online listening statistics.
- Reproducing upstream bugs or degrading working local metadata corrections.
- Altering the user's real reference playlists to test destructive writes.
- Changing unrelated work already present in the shared checkout.

## Reference and evidence rules

Reference checkout: `/private/tmp/lumilio-yesplaymusic-reference`, clean at
`df075cca247eab7bf8686155cb8cc9a1f4c7e271`, upstream
<https://github.com/qier222/YesPlayMusic>.
Browser reference: `http://localhost:20201/library`, signed in, dark appearance.
Current target: `http://localhost:6657/music?view=albums`.

`UI` below means the page or dialog was actually inspected. `Source` means
source establishes behavior but its completed interaction has not yet been
verified in the browser. A visible action is not proof its mutation succeeds.
Screenshots are reference evidence, not permission to use personal artwork or
account data as fixtures.

## Visual inventory

| Surface | Specification | Evidence |
| --- | --- | --- |
| Shell | Fixed 64px top navigation; back/forward at left, centered navigation, search and avatar at right; no permanent sidebar | UI; `App.vue`, `NavBar.vue` |
| Scroll | One main scroll surface, padding `64px 10vw 96px`; horizontal padding becomes `5vw` at width <=1336px | UI; `App.vue:118` |
| Typography | Barlow with system fallbacks; Library title 42px/700, avatar 44px with 12px gap | UI; `global.scss`, `library.vue:421` |
| Highlights | 3:7 flex weights, right list margin-left 36px; section top margin 24px plus 8px child margin | UI; `library.vue:435` |
| Likes card | Radius 16px, padding 18px 24px, lyrics area above bottom title/count; title 24px, count 15px; 44px play circle | UI; `library.vue:446` |
| Compact tracks | Three columns, row-major order, 4px grid gap; 36px covers, 14px cover-to-text gap, 16px title and 12px artist | UI; `TrackList.vue:174`, `TrackListItem.vue:401` |
| Categories | Section margin-top 54px, bottom gap 24px; 18px/600 labels, 8px 14px padding, 14px right margin, 8px radius; selected filled neutral background | UI; `library.vue:516` |
| Cover grid | Five columns, `44px 24px` gaps, square covers with 12px radius; 16px/600 title, 12px subtitle | UI; `CoverRow.vue`, `Cover.vue` |
| Artist grid | Same five columns; circular images, centered name without track/album count underneath | UI; `CoverRow.vue` |
| Hover play | Centered translucent circular button sized as percentage of cover; image-colored blurred shadow; cover itself opens details, play button stops navigation | UI hover seen; source `Cover.vue` |
| Context menu | Anchored at pointer, constrained to viewport and player; 136–240px width, 12px radius, 6px inset, 14px/600 items padded 10px 14px, separators | UI; `ContextMenu.vue` |
| Create modal | Centered compact modal, title and close control, one title input, privacy checkbox, full-width primary Create footer | UI; `ModalNewPlaylist.vue` |
| Add-to-playlist modal | Same compact modal; New Playlist button first, then own playlists with 42px cover, title, track count; click row submits | UI; `ModalAddTrackToPlaylist.vue` |
| Playlist detail | 288px cover; text column gap 56px, title and creator/date/count, Play and more actions; wide full track list below | UI; `playlist.vue` |
| Liked detail | Avatar and large owner title, compact search trigger upper right, full single-column tracks with album, heart and duration; no persistent highlights | UI; `playlist.vue` |
| Album detail | 288px cover, 56px gap, 56px title, subtitle, artist, year/count/duration; actions; disc groups, release information and more by artist | UI and `album.vue` |
| Artist detail | Circular portrait, name, linked counts, expandable biography, Play/Follow/more; latest release, popular tracks, releases, MV, related artists | UI and `artist.vue` |
| Player | 64px bottom dock, full-width 2px seek bar on top; artwork/title/artist/heart left, previous/play/next centered, queue/repeat/shuffle/volume/lyrics right | UI and `Player.vue` |

## Interaction inventory

| ID | Entry and action | Result / ownership | Evidence |
| --- | --- | --- | --- |
| L01 | Click Library category | Changes local active tab, retains mounted panels via `v-show`, scrolls main to 375px smoothly | UI scroll observed; `library.vue:updateCurrentTab` |
| L02 | Click playlist label | Selects playlist panel; distinct from arrow target | UI; `library.vue` |
| L03 | Click playlist arrow | All / My / Liked menu; selects owner filter | UI menu; filter code `filterPlaylists`; post-filter refresh needs recheck |
| L04 | Click likes card | Navigates to `/library/liked-songs` | UI |
| L05 | Click likes play circle | Mode menu: liked songs / intelligence mode; menu selection begins playback | UI liked-mode play verified; cardiac mode excluded |
| L06 | Double-click compact/full track | Plays the owning list starting at selected track | Source; UI pending |
| L07 | Click track cover / artist / album | Cover opens album; artist and album text navigate to their respective details | Source and links observed |
| L08 | Right-click track | Track artwork/title header, Play, Add to queue, like/unlike, Add to playlist, Copy URL; extra removal item according to list context | UI liked-track menu; `TrackList.vue` |
| P01 | New Playlist | Title maxlength 40; optional privacy toggle; Create posts then closes, shows success and refetches owned playlists | UI dialog; mutation Source |
| P02 | Track menu → Add to playlist | Modal lists own playlists, excluding special likes playlist; row adds track and closes with feedback | UI dialog; mutation Source |
| P03 | Add modal → New Playlist | Carries selected track into create operation, then adds it to newly created playlist | Source |
| P04 | Open own playlist / more or right-click title | Menu includes library toggle, search, edit and delete; own header hides library heart | UI; `playlist.vue` |
| P05 | Edit playlist information | Upstream placeholder calls native alert: feature under development; **no edit form exists** | Source; UI invocation blocked browser operations |
| P06 | Delete owned playlist | Native confirmation before DELETE; success alert then history back | Source; no real playlist deleted |
| P07 | Remove track from owned playlist | Context action sends `op: del`, removes displayed row after success | Source; no real entry removed |
| P08 | Search in playlist | Toggles inline input; searches loaded metadata, loads additional tracks | Source; UI pending |
| P09 | Click description | Full-description modal; outside dismissal | Source |
| A01 | Library Albums | Saved albums grid, artist subtitle links | UI |
| A02 | Album heart / menu | Save/remove album preference; does not delete audio | Source; UI pending |
| A03 | Album menu Add to playlist | Visible item has **no handler** in reference version | Source; do not claim working CRUD |
| R01 | Library Artists | Followed artists grid; open artist, hover play | UI grid; detail pending |
| R02 | Artist Follow | Toggles followed preference; does not delete artist or tracks | Source |
| V01 | Library MVs | Wide thumbnail cards with titles and creators | UI; online scope excluded from local implementation |
| C01 | Library Cloud Disk | Compact tracks and Upload Songs action; account currently renders empty body | UI; upload and removal Source only |
| H01 | Library Play History | Latest Week / All Time toggles; All Time produced a single-column list with play counts | UI |
| Q01 | Player queue | Navigates to next-tracks page with now playing, inserted queue and next-up; inserted queue supports remove/clear | Source `next.vue` |
| Q02 | Transport | Previous/next, play/pause, seek, mute/restored volume, repeat modes, shuffle | Source `Player.vue`; UI pending |
| Q03 | Lyrics | Opens full lyric surface; local replica must use existing stored lyrics | UI full lyric surface opened and closed |

## Local capability mapping

- Likes use Asset-owned likes; album/artist favorites use the existing Music
  preferences. The highlight list must query liked tracks, not recent imports.
- Playlist create/read/update/delete and entry add/remove/reorder already have
  typed Music endpoints. Existing metadata editing remains real and can live
  behind the matching more menu, despite the upstream unfinished edit action.
- Local albums/artists are catalog entities, not online saved subscriptions.
  Any All/Favorites access must remain usable without introducing a permanent
  extra filter toolbar absent from the reference.
- No online MV/cloud/history replacement should be silently fabricated. Audit
  these categories fully, but implement only honest local equivalents within
  the established product scope.
- Queue is transient playback state, not a saved playlist. Keep duplicate local
  entries distinct and preserve the persistent shell-owned audio engine.
- No track-row action may delete original files as a substitute for removing a
  playlist entry, favorite, or queue entry.

## Execution phases

### Phase 0 — Finish reference inventory

- [x] Read referenced task and its supplied screenshot.
- [x] Inspect all six Library categories, liked detail, own playlist detail,
  create/add dialogs, playlist filters and context menus.
- [x] Cross-check the source version, layout constants and CRUD handlers.
- [x] Recover reference tab after native edit alert blocks browser commands.
- [x] Inspect album/artist details and menus; playback, queue and lyric states.
- [x] Finish target comparison and freeze allowed local deviations.

### Phase 1 — Rebuild shared presentation

Music navigation shell, Library layout, cards, rows, menus and dialogs based
on the frozen evidence, preserving local data and route/state ownership.

### Phase 2 — Wire local interactions

Detail navigation, likes/favorites, playlist CRUD, row playback/context menus,
search, queue and lyric presentation; meaningful regression coverage.

### Phase 3 — Verify

Browser comparison of all mapped states, focused behavior tests, Web gate,
i18n extraction/coverage, generated feature docs and architecture boundaries.

## Validation boundaries

- Matching desktop geometry and theme, including column order, spacing,
  hover/active states, menus, player and scroll behavior.
- Every visible action has a working local result; create/edit/add/remove
  preserve identity and update the appropriate list; cancellation is safe.
- Music playback survives route transitions, seeks correctly, and restores
  paused state after refresh without stealing another media viewer's audio.
- Empty, unavailable and long-title states are deliberate; mobile navigation,
  menus and player remain accessible without horizontal overflow.
- Completion must name remaining deviations; unverified reference states stay
  explicitly pending, not converted to pass based on source alone.

## Implementation checkpoint

Music routes now use reference navigation geometry, Barlow (SIL OFL bundled),
neutral theme colors, a playlist-first Library, 12 liked highlights, five-column
covers with separate hover-play targets, double-click track playback, native
popover track actions, split create/add dialogs, entity detail heroes, a 64px
player, queue surface and fullscreen local lyrics. Existing playlist editing
and entry ordering are behind More options. Local catalog tabs remain
Playlists / Albums / Artists / Tracks; filters are All / Favorites. No online
category or private/public toggle is fabricated.

Validation in progress: two focused dialog regressions, Web gate, browser
create/add/edit/remove, typography/layout and transport. Exact reference tab
scroll behavior, lyric excerpt on the likes card, artist releases, and detail
menu parity still require completion; do not claim full 1:1 parity yet.

## Verification on 2026-09-08

- Type check: 732 files, pass. Lint and source boundaries pass.
- Unit suite: 58 files / 304 tests, pass. Chinese translation coverage 100%.
- Playlist dialog integration: 2 tests pass; restoring duplicate creation
  caused the retry regression to fail with 2 creates versus expected 1.
- Earlier complete Web run: 101 files passed, 2 skipped; sole failure was
  the blanket Library terminology prohibition. The scoped terminology test
  subsequently passed, and all 304 unit tests passed after the correction.
- Browser: created local disposable playlist, added two tracks, moved the
  second above the first, renamed, refreshed and observed both persisted,
  removed one entry via row menu, double-clicked remaining track to play,
  paused, opened/closed full lyrics and queue, returned to Library.
- Local verification playlist: `0035ee12-b121-45f2-96ab-a26bdaaa4a9a`, named
  `Library parity verified`; retained for review (one Light Chamber entry).
- Reference-account mutations were not performed. No original media deleted.
- Final architecture check is blocked by approval-service quota. The tool
  explicitly rejected escalation for Go build-cache access and prohibited
  workarounds. Full Web/E2E final rerun remains outstanding; do not claim all
  gates passed. Generated feature docs were refreshed with `task web:docs`.
- Known parity limitations: artist portraits/release sections are unavailable
  in the current local artist DTO; playlist covers lack cover metadata; queue
  is a full-page overlay rather than a URL route; advanced reference online
  categories, intelligence playback and public/private controls are excluded.

## Music feature review on 2026-09-09

- Preserve the current shared-shell and unified track-list direction.
- Fixed duplicate playlist entry keys, hidden unresolved entries without a remove
  action, pagination after removing the last page, and the forced-open mobile sidebar.
- Added Asset-owned 0–5 ratings to the Music DTO and detail/fullscreen controls.
- Playlist covers derive from the first live owner-scoped entry with medium artwork;
  queries resolve this without loading every playlist's entries into the browser.
- Fullscreen playback displays the existing waveform derivative with seeking.
- Local LRC timestamps support repeated stamps, offsets, highlighting and seek.
  Plain text remains supported. Editing retains the original revision and draft
  across background refetches instead of silently replacing unsaved lyrics.
- Custom uploaded playlist artwork, LRC file import, rating filters, playback
  history and smart playlists remain potential future extensions.
- Validation pending in this review session; earlier verification is historical.

### Review validation and remaining boundaries

- `task server:test`: passed, including ratings and derived-playlist-cover checks.
- `task web:test`: 103 files passed, 2 skipped; 423 tests passed, 6 skipped.
- Added Chromium regressions cover rating save/clear/error, timed lyric seeking,
  and draft/revision retention across a background refetch.
- Artist detail sorting now requests the global server order before paging and
  carries the same search/sort into playback. Its browser regression failed
  against the original page-only sorting and passed after the fix.
- Positive LRC offsets advance lyrics; the offset tests failed with addition
  and pass with subtraction. Reference: https://github.com/Clarkkkk/paroles .
- Fixed the pre-existing architecture-gate violation in embedded-artwork
  extraction by using the configured ToolSession thread arguments. Architecture
  and the processors package pass after this change.
- Shared detail toolbars and ratings wrap on narrow screens; unavailable
  playlist entries remain removable and use the current search filter.
- The old localhost:6657 preview was not running. Live full-stack browser E2E
  was not executed in this session; Chromium component/flow tests use MSW.

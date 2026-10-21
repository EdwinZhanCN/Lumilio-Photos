# Decision: Share attributed Bandcamp originals between demo and E2E

Status: implemented

## Decision

Lumilio-Assets owns one public music corpus: 35 freely downloaded Bandcamp
tracks across nine albums, published as `assets-v1.3.0`. Demo and E2E profiles
reference the same immutable originals through Git LFS. The catalog records
artist, title, source album/track URL, license, expected tags, encoding and
integrity; MUSIC-SOURCES.md carries the human-readable attribution.

The selected pages link CC BY 4.0. Additional CC0 statements and the Anthology
BY 3.0 description are retained explicitly rather than labeling the entire
corpus CC0. Purchased Music downloads are excluded. Audio bytes, tags and
embedded artwork are preserved, with no derived format copies in the pool.

Demo seeding waits for exact successful upload receipts, then for extracted
Music metadata. It creates source albums using catalog `source.albumUrl` and
public Music APIs, assigning only ungrouped tracks with optimistic revisions.
Retries preserve existing assignments. Ordinary source files lack stable
release IDs, so title-only automatic album grouping is not assumed.

## Alternatives

A private purchased-audio overlay would split local demos from reproducible CI
and cannot supply redistributable public fixtures. Generated tones alone do not
exercise real encoders, tags, attribution and release variants. Transcoding
originals to manufacture every format would obscure source behavior. Profiles
instead select existing originals covering AAC, ALAC, FLAC, MP3, Vorbis and AIFF.

Asset counts cannot prove seed completion: unrelated files and delayed metadata
can satisfy a count while the intended tracks are absent. Receipt identity and
expected Music metadata provide the required readiness evidence.

## Evidence

Full decode of 35 tracks, catalog integrity for 301 assets and Git LFS fsck
passed. A real Music demo import and retry each finished with 35 tracks and
nine albums. Server and Web gates passed (Web: 416 passed, six skipped).
The Music browser regression covers original metadata, explicit edition
separation, album order, persistent playback across SPA navigation and close.
It reproduced a null event.currentTarget crash; the player now captures audio
time/duration synchronously before deferred state updates. Runtime startup
also exposed and fixed the Gin :id/:trackId wildcard conflict.

Embedded audio-cover thumbnail extraction remains tracked in the tech debt
list; demo albums use placeholder artwork until the pipeline supports it.

The user explicitly approved producer publication. Assets `main` and annotated
`assets-v1.3.0` were pushed at revision
`2e5439f77e562706859ef82166ab9a452c603d67`, with all 35 LFS objects uploaded.
`task assets:reconcile RELEASE=assets-v1.3.0`, `task assets:check` and
`task assets:verify` passed. Both smoke and complete demo profiles were
materialized through the committed sync path. The final pinned Music E2E
passed (one test, 5.8 seconds), using replay fakelumen and keyless fake Ollama;
the isolated stack and its disposable volumes were removed afterward.

# Postmortem 0003: Web media URLs switched files under a playing element

## Executive summary

`/assets/{id}/audio/web`, `/assets/{id}/video/web` and the public-share
`web-audio`/`web-video` URLs served the original file until the web transcode
was published, then served the transcode from the same URL. A media element
keeps range-requesting its source URL for its whole lifetime and never
revalidates the entity, so a track that began playing as its original
FLAC/AAC received MP3 bytes, or a `416` past the MP3's end, on its next range
request after the transcode landed. Playback then hung forever: no `error`,
no `ended`, the player still showing Pause. Newly imported music is the
common case, because transcoding runs after upload operations report
success. The unpinned URL now redirects (`307`, `no-store`) to
`?variant=web|original`, and a pinned URL only ever serves that
representation.

## What broke

`openWebOrOriginalWithFallback` chose the representation per request. Seeking
past the buffered range of a freshly imported 21 MB FLAC after its MP3 was
published issued `Range: bytes=17…-` and got
`416 bytes */4837090`; the element stayed at the seek target, unpaused and
waiting, indefinitely. Ranges that fall inside the MP3 would instead splice MP3
bytes into the FLAC stream. The responses also carried
`Cache-Control: public, max-age=86400` with no validator identifying the
representation, and Chromium does not send `If-Range` for media ranges, so
validators alone could not have prevented the splice.

## Why every net missed it

- The handlers had no tests; the fallback was reasoned about per request, not
  across the lifetime of one media element.
- Local and CI stacks buffer these small files in one response, so the
  playback E2E specs never issue a second range request. The window only opens
  with a seek, a slow link, or a large file while the transcode backlog drains.
- The music E2E specs observe that playback *starts*, which is exactly the part
  that works.

## Guardrails added

- [`TestServePinnedWebMediaKeepsRangesOnTheRepresentationPlaybackStartedWith`](../../server/internal/api/handler/media_paths_test.go)
  publishes the MP3 between range requests and requires the pinned original to
  keep serving, including past the MP3's length. It runs under `task server:test`.
- All four web-media endpoints share `servePinnedWebMedia`, so a new web-media
  route inherits the pinning instead of re-deriving the fallback.

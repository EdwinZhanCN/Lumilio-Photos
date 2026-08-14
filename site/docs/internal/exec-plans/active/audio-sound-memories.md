# Audio as Sound Memories

Status: active, not started. Product direction and contracts frozen 2026-08-14.

Goal: give AUDIO assets a product home as **sound memories** — voice memos,
family recordings, oral stories, field recordings — for NAS home users, with
photographers/designers storing project audio as a side benefit. Audio becomes
reachable again through a dedicated Recordings surface, gains Whisper-based
transcription as an optional Lumen capability, becomes text-searchable next to
OCR, and becomes usable by the Lumilio agent.

Audio is **not** a fourth media pillar. It is explicitly not a music library
(Navidrome/Plexamp territory: playlists, artist/album browse, Subsonic,
scrobbling, offline sync) and not a pro-audio tool (DAW drag-drop, UCS, BPM/key
analysis). Those are separate products and remain non-goals forever, not just
for this plan.

## Current baseline (verified 2026-08-14)

Backend is nearly complete for storage/processing:

- `internal/utils/file/validator.go` accepts 10+ audio formats
  (mp3/aac/m4a/flac/wav/ogg/aiff/wma/opus).
- `processors/audio_helpers.go`: ffprobe probe, exiftool metadata with ID3
  tags (`AudioSpecificMetadata`: codec/bitrate/sample-rate/channels/artist/
  album/title/genre/year), smart web-MP3 transcode (`transcodeAudioSmart`),
  waveform PNG saved as thumbnail version `"waveform"`.
- Agent `asset_filter` already accepts `type=AUDIO`; duplicate/share/trash
  services are type-agnostic.

Frontend components exist but are unreachable:

- `AudioInfoView`, `MediaThumbnail` audio branch, Vidstack audio player CSS,
  `PublicShareGrid`/`PublicShareLightbox` audio handling all exist.
- `DEFAULT_ASSET_TYPES` queries photos+videos only; `type=audio` is rejected by
  `browseRouteState`; `AssetUserFilter.type` is narrowed to `PHOTO | VIDEO`.
  Uploaded audio is processed but invisible — a silent black hole.

Known gaps this plan owns:

- `exif/parsers.go` `parseCommonMetadata` has no AUDIO branch → audio never
  gets `TakenTime`, so recordings cannot sit on the timeline or join Events.
- The waveform thumbnail is stored but unreachable: the thumbnail endpoint
  validates `size ∈ {small, medium, large}`.
- No transcription, no transcript storage/search, no agent visibility into
  audio content (`inspect` falls through to a camera-only default branch).

## Decisions (frozen)

1. **UI form: a dedicated Recordings destination under Collections, not a
   photos/videos tab.** Route `/collections/recordings` (+ `/:assetId` viewer),
   registered like Liked/Trash. The flow lives in the assets feature
   (`web/src/features/assets/flows/recordings/`, route component
   `features/assets/routes/Recordings.tsx`) because it is an asset-browsing
   journey and the full-screen viewer lives there; Collections hub gets a card.
   No new top-level sidebar entry; the Assets timeline stays photos+videos and
   the browse filter chips keep excluding AUDIO.
2. **Presentation is list-first, not a masonry grid.** Rows grouped by month:
   waveform image, title (ID3 title, else filename), duration, date
   (`taken_time` else `upload_time`), transcription status. Selection reuses
   `lib/assets/bulkActions` (share/album/like/trash). Clicking a row opens the
   existing full-screen viewer route. Phase 1 may ship on `AssetBrowser` with a
   `{ type: "AUDIO" }` constraint to get the viewer wiring for free, but the
   list layout is the Phase 1 exit state.
3. **Transcription is a new optional Lumen capability**, never an embedded
   model. Internal service name `whisper`, protocol task `audio_transcribe`.
   Canonical user-facing capability terminology (must be added to the
   [lumilio-frontend-i18n](../../../../../.agents/skills/lumilio-frontend-i18n/SKILL.md)
   table and `productTerminology.test.ts` when Phase 2 ships): `语音转写` /
   `Voice Transcription`. ML stays optional: with no Hub, recordings remain
   fully browsable/playable.
4. **Transcript search mirrors the OCR sidecar exactly.** SQLite rows are
   authoritative; a revision outbox feeds a rebuildable Bleve index at
   `<sqlite-dir>/indexes/bleve/transcript-v1/`; a new RRF source
   `SourceTranscript` joins aggregate retrieval and agent set-retrieve. Bleve
   documents are per-segment so a hit carries the matching segment's
   `start_ms` into `Candidate.BestTsMs` (field shipped by video-semantic-search)
   and the player seeks to the matching sentence.
5. **Agent integration is three seams, no new subsystem:** extend the
   `search_text` producer to query OCR + transcript indexes; add an AUDIO
   branch to `inspect`; add one new observer tool `read_transcript` that
   returns timestamped transcript text for ≤2 assets so the agent can
   summarize and quote recordings. Everything else (albums, tags, show, share)
   already works on refs containing AUDIO.
6. **Recordings join Events through metadata, not special cases.** Adding the
   AUDIO branch to `parseCommonMetadata` gives voice memos their real
   `TakenTime`; the Event resolver picks them up like any media item.

## Phase 1 — Recordings surface (no ML)

Backend:

1. `exif/parsers.go`: add `case dbtypes.AssetTypeAudio` to
   `parseCommonMetadata` using `CreateDate` / `MediaCreateDate` /
   `TrackCreateDate` / `DateTimeOriginal` (voice-memo m4a carries QuickTime
   dates). Missing tags → `TakenTime` stays nil and the UI falls back to
   upload time, as `AudioInfoView` documents today.
2. Thumbnail endpoint: for AUDIO assets resolve the `"waveform"` thumbnail
   row regardless of requested size (and accept explicit `size=waveform`).
   Update the Swagger enum + `task dto`.
3. Verify `/api/v1/assets/list` + search accept `type=AUDIO` end-to-end
   (agent filter already does; widen the HTTP DTO if it is narrowed).
4. Verify the Event rebuild path includes AUDIO assets once `taken_time`
   exists; fix type gates if any exist.

Frontend:

1. `features/assets/flows/recordings/`: `RecordingsFlow` with month-grouped
   list, waveform via the thumbnail endpoint, bulk actions, breadcrumbs;
   `routes/Recordings.tsx`; routes `/collections/recordings(/:assetId)`;
   Collections hub card ("录音 / Recordings").
2. Upgrade `MediaThumbnail`'s audio branch to render the waveform image as
   card background (matters for Events/albums/share grids where audio now
   appears).
3. Keep `AssetUserFilter` narrowed; the page constraint (`AssetFilterDTO`
   with `type: "AUDIO"`) is the only audio query path.
4. i18n extract-then-fill; update the assets feature `doc.ts` flow list.

Exit: an uploaded voice memo is visible in Recordings, plays in the viewer,
shows correct capture date, can join albums/shares/events, and the library
has no invisible-but-accepted media type.

## Phase 2 — Transcription backbone

External contract (cross-repo, gates Phase 2 merge):

- Lumen SDK (additive, next backward-compatible tag; no committed `replace`):
  `types.TaskAudioTranscribe = "audio_transcribe"`, request builder taking
  audio bytes + mime + optional language hint, and `types.TranscriptionV1`:
  `{ model_id, language, segments: [{start_ms, end_ms, text}] }`.
- Lumen Hub: new `whisper` service advertising the `audio_transcribe` task
  contract (suggested runtime: faster-whisper / whisper.cpp; model choice is
  Hub-owned and opaque to Photos beyond `model_id`).
- Desktop-managed Lumen packaging follows separately; Photos behavior must not
  depend on it (capability simply stays unavailable).

Photos backend:

1. `LumenService.AudioTranscribe(ctx, audioBytes, mime, langHint)` +
   disabled-impl stub; route through `FindTaskContract` like OCR/face.
2. Migration (next free number): `audio_transcripts` (PK `asset_id` →
   `assets`, `model_id`, `language`, `full_text`, `segment_count`,
   `processing_time_ms`, timestamps) + `audio_transcript_segments`
   (`asset_id`, `seq`, `start_ms`, `end_ms`, `text`) +
   `transcript_index_metadata` / `transcript_index_outbox` mirroring the
   `000002` OCR shapes. sqlc queries + `TranscriptService`
   (save = replace-all in one transaction + revision bump + outbox row).
3. Job `ProcessTranscribeArgs` (kind `process_transcribe`, ML insert opts),
   thin worker mirroring `ml_ocr_worker.go`: gate
   `isMLTaskEnabled("process_transcribe")`, load the web audio version (same
   resolution as the playback handler; original when the web version is a
   copy), call `AudioTranscribe`, save via `TranscriptService`. Skip with a
   typed log for durations > 4h (constant, not a setting). Queue
   `process_transcribe` `MaxWorkers: 1`; generous timeout (~20 min).
4. Enqueue after successful `transcodeAudioSmart` in `transcode_task.go`
   (same style as the video-frames enqueue), gated on effective settings;
   extend `asset_retry.go` for audio ML retry.
5. Settings: `ML.TranscribeEnabled` (prod default true, dev zero-value false —
   same pattern as other ML toggles), wired through
   settings migration/sqlc/DTO/`isMLTaskEnabled`/`HasManualTasksEnabled`;
   Settings AI tab toggle; Monitor capability card using the canonical
   `语音转写` / `Voice Transcription` label; add the terminology-table row.
6. Indexing/backfill: `AssetIndexingTaskTranscribe = "transcribe"` counting
   AUDIO assets missing an `audio_transcripts` row; Monitor progress bucket;
   reset path re-enqueues audio.
7. HTTP: `GET /api/v1/assets/{id}/transcript` → `TranscriptDTO`
   (language, model_id, full_text, segments); 404 when absent; `task dto`.

Exit: with a whisper-capable Hub, an uploaded voice memo gets a stored,
API-readable transcript; toggling the capability off stops new jobs without
breaking browse/playback.

## Phase 3 — Transcript search

1. `server/internal/search/blevetranscript` mirroring `bleveocr`
   (document/mapping/index/query/writer/rebuild): one document per segment,
   id `{asset_id}#{seq}`, fields `asset_id`, `text` (same CJK tokenization
   strategy as OCR), stored `start_ms`.
2. Outbox drain worker + queue (`transcript_index`, `MaxWorkers: 1`) mirroring
   `search_ocr_outbox_worker.go`; startup rebuild parity with the OCR sidecar.
3. `SourceTranscript = "transcript"` retriever: max-pool per asset, best
   segment's `start_ms` → `Candidate.BestTsMs`; initial RRF weight equal to
   `SourceOCR`. Wire into aggregate retrievers, `setretrieve` (agent), and
   the search service.
4. Frontend: search hits on recordings open the viewer seeked to
   `best_ts_ms` (reuse the video seek path); the Recordings page search box
   goes server-side (type-scoped search) once this lands; viewer info panel
   gains a lazy-loaded transcript section with click-to-seek segments.

Exit: searching a phrase returns the recording ranked alongside photo OCR
hits and opens it at the matching sentence.

## Phase 4 — Agent integration

1. `search_text`: query OCR + transcript indexes, union with per-source
   dedupe; update tool description ("text in images or spoken words in
   recordings") so ref chains like *search → add_to_album* cover recordings.
2. `inspect`: AUDIO case emitting duration/codec plus sanitized
   artist/album/title/genre/year from `AudioSpecificMetadata`.
3. New observer `read_transcript`: input ref (≤2 AUDIO assets), returns
   `[mm:ss]`-prefixed segment lines, `SanitizeUserText`, hard output cap
   (~4k chars with truncation notice). This enables "总结这段录音 / 谁在
   什么时候提到了X" Q&A with timestamp citations.
4. Verify `show`/`peek` render audio refs sanely in chat (waveform cards via
   the Phase 1 `MediaThumbnail` upgrade); an inline chat audio player block is
   a follow-up, not this plan.

Exit: "找出所有提到奶奶的录音，加进'家史'相册，然后总结最长的一段" works
end-to-end through existing ref plumbing.

## Config

| Field | Prod default | Dev zero-value |
|-------|--------------|----------------|
| `ML.TranscribeEnabled` | `true` | `false` |

No sampling knobs; the 4h duration cap is a code constant. TOML is untouched
(runtime-mutable settings only), consistent with other ML toggles.

## Verification

- Phase 1: `task server:test` (parser AUDIO branch, waveform endpoint,
  list/search type filter), `task web:test` (unit for list grouping/model,
  component for row + waveform fallback, flow spec for
  `/collections/recordings` with MSW), `task dto`, i18n extract, `task
  ci:architecture`.
- Phase 2: worker save/replace semantics with fake Lumen; settings gate on/off;
  indexing counts; retry path. Manual: real Hub smoke on a short m4a + a >4h
  skip.
- Phase 3: outbox drain + rebuild parity tests (mirror `bleveocr` suites);
  retriever `BestTsMs` correctness; mixed OCR+transcript ranking regression.
- Phase 4: tool unit tests (cap, sanitization, ≤2-asset guard); an e2e-style
  agent conversation fixture if the harness allows.

## Non-goals

- Music-library features: playlists, artist/album/genre browse, Subsonic API,
  scrobbling, gapless, offline sync, music-tag editing.
- Pro-audio features: DAW integration, sample analytics (BPM/key), UCS.
- CLAP-style audio semantic embeddings; speaker diarization; translation.
- Transcript editing UI; TTS; chat inline audio player (follow-up candidate).
- Audio in the Assets timeline or its filter chips.
- Hub/desktop packaging of the whisper runtime (external plans).

## Critical files

- `server/internal/utils/exif/parsers.go` — AUDIO `parseCommonMetadata` branch
- `server/internal/api/handler/asset_handler.go` — waveform thumbnail; transcript endpoint
- `server/internal/processors/{transcode_task,asset_retry}.go` — enqueue + retry
- `server/internal/queue/{jobs/types.go,ml_transcribe_worker.go,queue_setup.go}` + `app/app.go`
- `server/internal/service/{lumen_service.go,transcript_service.go,settings_service.go,indexing_service.go}`
- `server/internal/search/blevetranscript/*`, `search/{types,retrievers,setretrieve,service}.go`
- `server/internal/agent/tools/{producers,inspect,read_transcript}.go`
- `server/migrations/0000XX_audio_transcripts.up.sql`
- `web/src/features/assets/{flows/recordings/*,routes/Recordings.tsx}`
- `web/src/features/assets/flows/browse/gallery/media/MediaThumbnail.tsx`
- `web/src/features/assets/flows/viewer/info/AudioInfoView.tsx` — transcript panel
- `web/src/app/router/routes.tsx`, Collections hub, Settings AI tab, Monitor

## Progress

- [ ] Phase 1 — Recordings surface
- [ ] Phase 2 — Transcription backbone (gated on SDK tag + Hub whisper)
- [ ] Phase 3 — Transcript search
- [ ] Phase 4 — Agent integration

## Decision log

- 2026-08-14: Audio positioned as sound memories for home users; music-library
  and pro-audio directions permanently rejected (market: Immich refuses audio
  while demand is memory-flavored; Navidrome ecosystem owns music; pro tools
  are desktop-native).
- 2026-08-14: Recordings lives under Collections as a list-first destination;
  Assets timeline remains photos+videos.
- 2026-08-14: Transcription is a Lumen capability (`whisper` /
  `audio_transcribe` / `语音转写` / `Voice Transcription`), never embedded.
- 2026-08-14: Transcript search reuses the OCR sidecar pattern with
  per-segment documents and `BestTsMs` seek; RRF weight starts equal to OCR.
- 2026-08-14: Agent surface = `search_text` extension + `inspect` AUDIO branch
  + `read_transcript` observer; no dedicated audio agent subsystem.

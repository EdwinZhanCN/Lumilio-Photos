/**
 * # Music
 *
 * Music owns the first-class local listening domain: audio tracks, release
 * albums, artist credits, playlists, and transient playback sessions. Asset
 * storage, likes, trash, and media delivery remain owned by Assets; this
 * feature stores only the music projection and user corrections.
 *
 * ## State
 *
 * TanStack Query owns catalog facts through the hooks in
 * {@link useMusicTracks}, {@link useMusicAlbums}, {@link useMusicArtists},
 * {@link useMusicPlaylists}, and their detail counterparts. URL parameters
 * select the browse view and search state. {@link MusicPlayerProvider} owns
 * the one authenticated audio engine, its queue cursor, and transient
 * controls. An owner-scoped browser snapshot restores the queue and seek
 * position in a paused state after refresh; no media URLs are persisted. The
 * queue is a playback-session projection, not a second saved playlist.
 *
 * ## Flows
 *
 * {@link MusicLibraryFlow} is the real-data browse surface for
 * tracks, albums, artists, and playlists destinations. The default playlist tab,
 * five-column covers, and 12 liked-track highlights follow the audited
 * YesPlayMusic browse surface. Browse tabs share one page-level scroll coordinate and
 * keep each results panel mounted while switching views. URL parameters still
 * select the active view and its filters.
 * Artwork failures keep stable placeholder dimensions. Detail flows keep
 * corrections and playlist editing behind explicit edit controls. Create and
 * add-to-playlist dialogs close after success; a failed initial add reuses the
 * newly created playlist on retry. Each
 * {@link MusicTrackRow} can start a server-backed snapshot, so playing a
 * search, album, or playlist is not limited to the rows currently rendered.
 *
 * ```mermaid
 * flowchart LR
 *   Browse --> Detail
 *   Browse --> Player
 *   Detail --> Player
 * ```
 *
 * ## Data
 *
 * {@link MusicPlayerDock} is composed once by the application shell beside
 * the outlet. A 64px dock opens the queue surface and native fullscreen lyrics
 * dialog without replacing the audio engine. Music routes use their own
 * navigation chrome while the shared shell preserves playback. The engine collects playback-session pages before publishing an
 * editable queue, keeps duplicate
 * playlist occurrences distinct, and coordinates with the neutral media
 * playback event used by Assets. Original audio files are never rewritten by
 * metadata corrections. Album/artist favorites are owner-scoped catalog
 * preferences. Local plain-text lyrics use optimistic revisions and never
 * require an online provider. Embedded artwork is generated through the
 * existing fenced Asset derivative pipeline alongside the waveform.
 *
 * @module
 */
import type MusicPlayerDock from "./components/MusicPlayerDock.tsx";
import type MusicTrackRow from "./components/MusicTrackRow.tsx";
import type {
  useMusicAlbums,
  useMusicArtists,
  useMusicPlaylists,
  useMusicTracks,
} from "./api/useMusic.ts";
import type MusicLibraryFlow from "./flows/browse/MusicLibraryFlow.tsx";
import type { MusicPlayerProvider } from "./state/MusicPlayerProvider.tsx";

export {};

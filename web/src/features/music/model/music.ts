import type { components } from "@/lib/http-commons";

export type MusicTrack = components["schemas"]["dto.MusicTrackDTO"];
export type MusicAlbum = components["schemas"]["dto.MusicAlbumDTO"];
export type MusicArtist = components["schemas"]["dto.MusicArtistDTO"];
export type MusicPlaylist = components["schemas"]["dto.MusicPlaylistDTO"];
export type MusicPlaylistEntry = components["schemas"]["dto.MusicPlaylistEntryDTO"];
export type MusicPlaybackEntry = components["schemas"]["dto.MusicPlaybackEntryDTO"];
export type MusicPlaybackSource = components["schemas"]["dto.MusicPlaybackSourceRequestDTO"];

export type MusicView = "overview" | "tracks" | "albums" | "artists" | "playlists";

export type MusicQueueItem = {
  track: MusicTrack;
  entryId?: string;
  available: boolean;
  savedTitle?: string;
};

export function trackId(track: MusicTrack): string {
  return track.track_id ?? "";
}

export function trackTitle(track: MusicTrack): string {
  return track.title?.trim() || track.original_filename?.trim() || "Untitled track";
}

export function trackArtist(track: MusicTrack): string {
  const credits = track.artists
    ?.map((credit) => credit.display_name?.trim())
    .filter((name): name is string => Boolean(name));
  return credits?.join(" · ") || track.artist_name?.trim() || "Unknown artist";
}

export function trackAlbum(track: MusicTrack): string {
  return track.album_title?.trim() || "Unknown album";
}

export function sortMusicTracks(
  tracks: MusicTrack[],
  sort: "" | "title" | "artist" | "album" | "track",
): MusicTrack[] {
  // "" = keep the source order (recently added).
  if (sort === "") return tracks;
  if (sort === "track") {
    return [...tracks].sort((a, b) => {
      const discA = a.disc_number ?? 0;
      const discB = b.disc_number ?? 0;
      if (discA !== discB) return discA - discB;
      return (a.track_number ?? 0) - (b.track_number ?? 0);
    });
  }
  const key = (track: MusicTrack) =>
    sort === "title"
      ? trackTitle(track)
      : sort === "artist"
        ? trackArtist(track)
        : trackAlbum(track);
  return [...tracks].sort((a, b) => key(a).localeCompare(key(b)));
}

export function formatMusicDuration(seconds?: number): string {
  if (seconds == null || !Number.isFinite(seconds) || seconds < 0) return "—";
  const minutes = Math.floor(seconds / 60);
  const remaining = Math.floor(seconds % 60);
  return `${minutes}:${remaining.toString().padStart(2, "0")}`;
}

export function playbackEntryToQueueItem(entry: MusicPlaybackEntry): MusicQueueItem | null {
  const id = entry.track_id;
  if (!id) {
    return {
      track: {
        track_id: undefined,
        title: entry.saved_title,
        artist_name: entry.track_artist,
        album_title: entry.track_album,
      },
      entryId: entry.source_entry_id ?? entry.entry_id,
      savedTitle: entry.saved_title,
      available: false,
    };
  }

  return {
    track: {
      track_id: id,
      title: entry.track_title || entry.saved_title,
      artist_name: entry.track_artist,
      album_title: entry.track_album,
      mime_type: entry.mime_type,
      duration: entry.duration,
      is_deleted: !entry.available,
    },
    entryId: entry.source_entry_id ?? entry.entry_id,
    savedTitle: entry.saved_title,
    available: entry.available ?? false,
  };
}

export function nextQueueIndex(
  currentIndex: number,
  length: number,
  direction: 1 | -1,
  repeat: "off" | "all",
): number | null {
  if (length <= 0) return null;
  const next = currentIndex + direction;
  if (next >= 0 && next < length) return next;
  if (repeat === "all") return direction === 1 ? 0 : length - 1;
  return null;
}

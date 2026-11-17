import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export const MUSIC_TRACKS_QUERY_KEY = ["get", "/api/v1/music/tracks"] as const;
export const MUSIC_ALBUMS_QUERY_KEY = ["get", "/api/v1/music/albums"] as const;
export const MUSIC_ARTISTS_QUERY_KEY = ["get", "/api/v1/music/artists"] as const;
export const MUSIC_PLAYLISTS_QUERY_KEY = ["get", "/api/v1/music/playlists"] as const;

export type MusicTrackQueryOptions = {
  query?: string;
  artistId?: string;
  sort?: "" | "title" | "artist" | "album" | "track";
  likedOnly?: boolean;
  limit?: number;
  offset?: number;
  enabled?: boolean;
};

export function useMusicTracks(options: MusicTrackQueryOptions = {}) {
  return $api.useQuery(
    "get",
    "/api/v1/music/tracks",
    {
      params: {
        query: {
          query: options.query,
          artist_id: options.artistId,
          sort: options.sort,
          liked_only: options.likedOnly,
          limit: options.limit ?? 50,
          offset: options.offset ?? 0,
        },
      },
    },
    { enabled: options.enabled ?? true, staleTime: 30_000 },
  );
}

export function useMusicTrack(trackId?: string, initialData?: import("../model/music").MusicTrack) {
  return $api.useQuery(
    "get",
    "/api/v1/music/tracks/{id}",
    { params: { path: { id: trackId ?? "" } } },
    { enabled: Boolean(trackId), staleTime: 30_000, initialData },
  );
}

export function useMusicAlbums(
  options: {
    query?: string;
    favoritesOnly?: boolean;
    limit?: number;
    offset?: number;
    enabled?: boolean;
  } = {},
) {
  return $api.useQuery(
    "get",
    "/api/v1/music/albums",
    {
      params: {
        query: {
          query: options.query,
          favorites_only: options.favoritesOnly,
          limit: options.limit ?? 50,
          offset: options.offset ?? 0,
        },
      },
    },
    { enabled: options.enabled ?? true, staleTime: 60_000 },
  );
}

export function useMusicAlbum(albumId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/music/albums/{id}",
    { params: { path: { id: albumId ?? "" } } },
    { enabled: Boolean(albumId), staleTime: 30_000 },
  );
}

export function useMusicArtists(
  options: {
    query?: string;
    favoritesOnly?: boolean;
    limit?: number;
    offset?: number;
    enabled?: boolean;
  } = {},
) {
  return $api.useQuery(
    "get",
    "/api/v1/music/artists",
    {
      params: {
        query: {
          query: options.query,
          favorites_only: options.favoritesOnly,
          limit: options.limit ?? 50,
          offset: options.offset ?? 0,
        },
      },
    },
    { enabled: options.enabled ?? true, staleTime: 60_000 },
  );
}

export function useMusicArtist(artistId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/music/artists/{id}",
    { params: { path: { id: artistId ?? "" } } },
    { enabled: Boolean(artistId), staleTime: 30_000 },
  );
}

export function useMusicPlaylists(
  options: { limit?: number; offset?: number; enabled?: boolean } = {},
) {
  return $api.useQuery(
    "get",
    "/api/v1/music/playlists",
    {
      params: {
        query: { limit: options.limit ?? 50, offset: options.offset ?? 0 },
      },
    },
    { enabled: options.enabled ?? true, staleTime: 30_000 },
  );
}

export function useMusicPlaylist(playlistId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/music/playlists/{id}",
    { params: { path: { id: playlistId ?? "" } } },
    { enabled: Boolean(playlistId), staleTime: 30_000 },
  );
}

export function useMusicPlaylistEntries(playlistId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/music/playlists/{id}/entries",
    { params: { path: { id: playlistId ?? "" } } },
    { enabled: Boolean(playlistId), staleTime: 15_000 },
  );
}

export function useMusicMutations() {
  const queryClient = useQueryClient();
  const invalidateMusic = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/tracks"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/tracks/{id}"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/albums"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/albums/{id}"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/artists"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/artists/{id}"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/playlists"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/playlists/{id}"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/music/playlists/{id}/entries"] }),
    ]);
  };

  const updateTrackMutation = $api.useMutation("patch", "/api/v1/music/tracks/{id}");
  const resetTrackMutation = $api.useMutation("post", "/api/v1/music/tracks/{id}/reset-overrides");
  const designationMutation = $api.useMutation("put", "/api/v1/music/tracks/{id}/designation");
  const updateAlbumMutation = $api.useMutation("patch", "/api/v1/music/albums/{id}");
  const updateArtistMutation = $api.useMutation("patch", "/api/v1/music/artists/{id}");
  const createPlaylistMutation = $api.useMutation("post", "/api/v1/music/playlists");
  const updatePlaylistMutation = $api.useMutation("patch", "/api/v1/music/playlists/{id}");
  const deletePlaylistMutation = $api.useMutation("delete", "/api/v1/music/playlists/{id}");
  const addEntryMutation = $api.useMutation("post", "/api/v1/music/playlists/{id}/entries");
  const removeEntryMutation = $api.useMutation(
    "delete",
    "/api/v1/music/playlists/{id}/entries/{entryId}",
  );
  const reorderMutation = $api.useMutation("put", "/api/v1/music/playlists/{id}/entries/reorder");

  return {
    updateTrack: updateTrackMutation,
    resetTrack: resetTrackMutation,
    setDesignation: designationMutation,
    updateAlbum: updateAlbumMutation,
    updateArtist: updateArtistMutation,
    createPlaylist: createPlaylistMutation,
    updatePlaylist: updatePlaylistMutation,
    deletePlaylist: deletePlaylistMutation,
    addEntry: addEntryMutation,
    removeEntry: removeEntryMutation,
    reorder: reorderMutation,
    invalidateMusic,
  };
}

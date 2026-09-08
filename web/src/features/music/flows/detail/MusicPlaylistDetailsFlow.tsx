import MusicMoreMenu from "../../components/MusicMoreMenu";
import { useMusicFeedback } from "../../state/useMusicFeedback";
import { useEffect, useState, type FormEvent } from "react";
import { ArrowDown, ArrowUp, ListMusic, Play, Save, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { useI18n } from "@/lib/i18n";
import {
  useMusicPlaylist,
  useMusicPlaylistEntries,
  useMusicMutations,
  useMusicTracks,
} from "../../api/useMusic";
import MusicTrackRow from "../../components/MusicTrackRow";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";

export default function MusicPlaylistDetailsFlow() {
  const { t } = useI18n();
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [ordering, setOrdering] = useState(false);
  const [searching, setSearching] = useState(false);
  const [entrySearch, setEntrySearch] = useState("");
  const { run, feedback } = useMusicFeedback();
  const { playlistId } = useParams<{ playlistId: string }>();
  const playlistQuery = useMusicPlaylist(playlistId);
  const entriesQuery = useMusicPlaylistEntries(playlistId);
  const [trackSearch, setTrackSearch] = useState("");
  const [trackOffset, setTrackOffset] = useState(0);
  const tracksQuery = useMusicTracks({ query: trackSearch, limit: 20, offset: trackOffset });
  const { playSource } = useMusicPlayer();
  const navigate = useNavigate();
  const { updatePlaylist, deletePlaylist, addEntry, removeEntry, reorder, invalidateMusic } =
    useMusicMutations();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [trackToAdd, setTrackToAdd] = useState("");
  const playlist = playlistQuery.data;
  const entries = entriesQuery.data?.items ?? [];

  useEffect(() => {
    if (!playlist) return;
    setTitle(playlist.title ?? "");
    setDescription(playlist.description ?? "");
  }, [playlist]);

  if (playlistQuery.isPending || entriesQuery.isPending)
    return (
      <div className="flex h-full items-center justify-center">
        <span className="loading loading-spinner loading-md text-primary" />
      </div>
    );
  if (playlistQuery.isError || entriesQuery.isError || !playlist)
    return (
      <div className="p-6">
        <div className="alert alert-error">
          {t("music.detail.error", "This music item could not be loaded.")}
        </div>
      </div>
    );

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await updatePlaylist.mutateAsync({
      params: { path: { id: playlist.playlist_id ?? "" } },
      body: { title, description, revision: playlist.revision },
    });
    await invalidateMusic();
    setEditing(false);
  };

  const addTrack = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const track = tracksQuery.data?.items?.find((item) => item.track_id === trackToAdd);
    if (!track?.track_id) return;
    await addEntry.mutateAsync({
      params: { path: { id: playlist.playlist_id ?? "" } },
      body: {
        track_id: track.track_id,
        revision: playlist.revision,
        idempotency_key: crypto.randomUUID(),
      },
    });
    setTrackToAdd("");
    await invalidateMusic();
  };

  const removeTrack = async (entryId: string) => {
    await removeEntry.mutateAsync({
      params: {
        path: { id: playlist.playlist_id ?? "", entryId },
        query: { revision: playlist.revision },
      },
    });
    await invalidateMusic();
  };

  const moveEntry = async (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= entries.length) return;
    const reordered = [...entries];
    const [moved] = reordered.splice(index, 1);
    if (!moved) return;
    reordered.splice(target, 0, moved);
    await reorder.mutateAsync({
      params: { path: { id: playlist.playlist_id ?? "" } },
      body: {
        revision: playlist.revision,
        entries: reordered.map((entry, position) => ({ entry_id: entry.entry_id ?? "", position })),
      },
    });
    await invalidateMusic();
  };

  const removePlaylist = async () => {
    if (
      !playlist.playlist_id ||
      !window.confirm(
        t("music.playlist.confirmDelete", "Delete this playlist? The audio files will be kept."),
      )
    )
      return;
    await deletePlaylist.mutateAsync({ params: { path: { id: playlist.playlist_id } } });
    await invalidateMusic();
    void navigate("/music?view=playlists", { replace: true });
  };

  return (
    <div className="music-browse music-scroll">
      {feedback}
      <div className="music-detail-hero playlist">
        <div className="music-detail-placeholder">
          <ListMusic className="size-24 opacity-30" />
        </div>
        <div>
          <h1>{playlist.title}</h1>
          <p className="text-sm opacity-60">
            {entries.length} {t("music.tracks.count", "tracks")}
          </p>
          {playlist.description && (
            <p className="mt-4 text-sm opacity-60">{playlist.description}</p>
          )}
          <div className="music-detail-actions">
            <button
              className="btn btn-primary"
              disabled={!entries.length}
              onClick={() => void playSource({ kind: "playlist", id: playlist.playlist_id })}
            >
              <Play className="size-4" fill="currentColor" />
              {t("music.player.play", "Play")}
            </button>
            <MusicMoreMenu
              actions={[
                {
                  label: t("music.playlist.edit", "Edit playlist"),
                  onSelect: () => setEditing(!editing),
                },
                {
                  label: t("music.playlists.addLabel", "Add to playlist"),
                  onSelect: () => setAdding(!adding),
                },
                {
                  label: t("music.searchTracks", "Search tracks"),
                  onSelect: () => setSearching(!searching),
                },
                {
                  label: t("music.playlist.entryHeading", "Playlist order"),
                  onSelect: () => setOrdering(!ordering),
                },
                {
                  label: t("music.playlist.delete", "Delete playlist"),
                  onSelect: () => {
                    void run(removePlaylist);
                  },
                  danger: true,
                },
              ]}
            />
          </div>
        </div>
      </div>
      <div className="w-full">
        <div className="w-full space-y-6">
          {editing && (
            <>
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  void run(() => save(event));
                }}
                className="grid gap-5 rounded-2xl border border-base-300/70 bg-base-100 p-5 sm:grid-cols-2 sm:p-6"
              >
                <label className="grid gap-2">
                  <span className="text-sm font-medium text-base-content/70">
                    {t("music.fields.playlistTitle", "Playlist title")}
                  </span>
                  <input
                    className="input input-bordered h-10 w-full px-3"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                  />
                </label>
                <label className="grid gap-2">
                  <span className="text-sm font-medium text-base-content/70">
                    {t("music.fields.description", "Description")}
                  </span>
                  <input
                    className="input input-bordered h-10 w-full px-3"
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </label>
                <button
                  type="submit"
                  className="btn btn-primary h-10 min-h-10 sm:col-span-2 sm:justify-self-end"
                  disabled={updatePlaylist.isPending}
                >
                  <Save className="size-4" />
                  {t("music.actions.save", "Save changes")}
                </button>
              </form>
            </>
          )}
          {adding && (
            <>
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  void run(() => addTrack(event));
                }}
                className="rounded-2xl border border-primary/20 bg-primary/5 p-5 sm:p-6"
              >
                <div className="mb-4">
                  <h2 className="text-lg font-semibold">
                    {t("music.playlists.addLabel", "Add to playlist")}
                  </h2>
                </div>
                <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(14rem,18rem)_auto] sm:items-end">
                  <label className="grid min-w-0 gap-2">
                    <span className="text-sm font-medium text-base-content/70">
                      {t("music.searchTracks", "Search tracks")}
                    </span>
                    <input
                      className="input input-bordered h-10 w-full px-3"
                      value={trackSearch}
                      aria-label={t("music.searchTracks", "Search tracks")}
                      placeholder={t("music.searchTracks", "Search tracks")}
                      onChange={(event) => {
                        setTrackSearch(event.target.value);
                        setTrackOffset(0);
                        setTrackToAdd("");
                      }}
                    />
                  </label>
                  <label className="grid min-w-0 gap-2">
                    <span className="text-sm font-medium text-base-content/70">
                      {t("music.playlists.chooseTrack", "Choose a track")}
                    </span>
                    <select
                      className="select select-bordered h-10 w-full"
                      value={trackToAdd}
                      onChange={(event) => setTrackToAdd(event.target.value)}
                    >
                      <option value="">{t("music.playlists.chooseTrack", "Choose a track")}</option>
                      {(tracksQuery.data?.items ?? []).map((track) => (
                        <option key={track.track_id} value={track.track_id}>
                          {track.title || track.original_filename}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    type="submit"
                    className="btn btn-primary h-10 min-h-10"
                    disabled={!trackToAdd || addEntry.isPending}
                  >
                    {t("music.playlists.add", "Add track")}
                  </button>
                </div>
                {tracksQuery.isError && (
                  <p role="alert" className="mt-3 text-sm text-error">
                    {t("music.detail.error", "This music item could not be loaded.")}
                  </p>
                )}
                <div className="mt-4 flex justify-between border-t border-base-300/60 pt-3">
                  <button
                    type="button"
                    className="btn btn-sm"
                    disabled={trackOffset === 0 || tracksQuery.isFetching}
                    onClick={() => {
                      setTrackOffset(Math.max(0, trackOffset - 20));
                      setTrackToAdd("");
                    }}
                  >
                    {t("common.previous", "Previous")}
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm"
                    disabled={
                      trackOffset + 20 >= (tracksQuery.data?.total ?? 0) || tracksQuery.isFetching
                    }
                    onClick={() => {
                      setTrackOffset(trackOffset + 20);
                      setTrackToAdd("");
                    }}
                  >
                    {t("common.next", "Next")}
                  </button>
                </div>
              </form>
            </>
          )}
          <section>
            {searching && (
              <input
                className="input mb-4 w-full"
                value={entrySearch}
                onChange={(event) => setEntrySearch(event.target.value)}
                aria-label={t("music.searchTracks", "Search tracks")}
                placeholder={t("music.searchTracks", "Search tracks")}
              />
            )}

            <div className="mb-3 flex items-center gap-2">
              <ListMusic className="size-5 text-info" />
              <h2 className="text-lg font-semibold">
                {t("music.playlist.entryHeading", "Playlist order")}
              </h2>
            </div>
            {entries.length === 0 ? (
              <p className="p-5 text-sm text-base-content/60">
                {t("music.playlist.empty", "This playlist has no entries yet.")}
              </p>
            ) : (
              <div className="space-y-1">
                {entries.map((entry, index) => (
                  <div
                    key={entry.entry_id ?? `${entry.position}-${index}`}
                    hidden={Boolean(
                      searching &&
                      entrySearch &&
                      ![entry.track?.title, entry.track?.artist_name, entry.saved_title]
                        .join(" ")
                        .toLocaleLowerCase()
                        .includes(entrySearch.toLocaleLowerCase()),
                    )}
                    className="flex items-center gap-1 rounded-xl border border-transparent hover:border-base-300/70"
                  >
                    {entry.track ? (
                      <div className="min-w-0 flex-1">
                        <MusicTrackRow
                          track={entry.track}
                          entryId={entry.entry_id}
                          onRemove={() => void run(() => removeTrack(entry.entry_id ?? ""))}
                          index={index}
                          source={{ kind: "playlist", id: playlist.playlist_id ?? "" }}
                        />
                      </div>
                    ) : (
                      <div className="min-w-0 flex-1 px-3 py-3 text-sm text-warning">
                        {entry.saved_title || t("music.playlist.unavailable", "Unavailable audio")}
                      </div>
                    )}
                    {ordering && (
                      <div className="flex shrink-0">
                        <button
                          type="button"
                          className="btn btn-ghost btn-xs btn-square"
                          onClick={() => void run(() => moveEntry(index, -1))}
                          disabled={index === 0 || reorder.isPending}
                          aria-label={t("music.playlist.moveUp", "Move up")}
                        >
                          <ArrowUp className="size-3.5" />
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost btn-xs btn-square"
                          onClick={() => void run(() => moveEntry(index, 1))}
                          disabled={index === entries.length - 1 || reorder.isPending}
                          aria-label={t("music.playlist.moveDown", "Move down")}
                        >
                          <ArrowDown className="size-3.5" />
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost btn-xs btn-square text-error"
                          onClick={() => void run(() => removeTrack(entry.entry_id ?? ""))}
                          disabled={!entry.entry_id || removeEntry.isPending}
                          aria-label={t("music.playlist.remove", "Remove entry")}
                        >
                          <Trash2 className="size-3.5" />
                        </button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

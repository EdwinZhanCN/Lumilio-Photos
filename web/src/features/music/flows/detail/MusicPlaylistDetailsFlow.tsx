import { useMusicFeedback } from "../../state/useMusicFeedback";
import { useEffect, useState, type FormEvent } from "react";
import { ListMusic, Play, Save, Shuffle } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { useI18n } from "@/lib/i18n";
import Modal from "@/components/ui/Modal";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMusicPlaylist, useMusicPlaylistEntries, useMusicMutations } from "../../api/useMusic";
import MusicArtwork from "../../components/MusicArtwork";
import MusicTrackList from "../../components/MusicTrackList";
import MusicPagination from "../../components/MusicPagination";
import MusicDetailToolbar from "../../components/MusicDetailToolbar";
import type { MusicSortValue } from "../../components/MusicSortDropdown";
import { trackAlbum, trackArtist, trackTitle } from "../../model/music";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";

export default function MusicPlaylistDetailsFlow() {
  const { t } = useI18n();
  const [editing, setEditing] = useState(false);
  const [sort, setSort] = useState<MusicSortValue>("");
  const [offset, setOffset] = useState(0);
  const [entrySearch, setEntrySearch] = useState("");
  const { run, feedback } = useMusicFeedback();
  const { playlistId } = useParams<{ playlistId: string }>();
  const playlistQuery = useMusicPlaylist(playlistId);
  const entriesQuery = useMusicPlaylistEntries(playlistId);
  const { playSource, shuffle, toggleShuffle } = useMusicPlayer();
  const navigate = useNavigate();
  const { updatePlaylist, deletePlaylist, removeEntry, reorder, invalidateMusic } =
    useMusicMutations();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const playlist = playlistQuery.data;
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("music.title"), to: "/music" },
    { label: t("music.views.playlists", "Playlists"), to: "/music?view=playlists" },
    { label: playlist?.title || t("music.views.playlists", "Playlists") },
  ]);
  const entries = entriesQuery.data?.items ?? [];
  // Available (resolved) playlist entries matching the in-list search. The
  // shared MusicTrackList renders these as a unified Tracks-style list; the
  // entry-level search-hide and unavailable (no track) fallback are handled
  // here / outside the list per the shared component's contract.
  const renderEntries = entries.filter((entry) => {
    if (!entry.track) return false;
    if (entrySearch) {
      const text = [entry.track?.title, entry.track?.artist_name, entry.saved_title]
        .join(" ")
        .toLocaleLowerCase();
      if (!text.includes(entrySearch.toLocaleLowerCase())) return false;
    }
    return true;
  });
  // Sort the *entries* (not just the tracks) so the entry id stays aligned with
  // each row regardless of sort — drag reorder / remove / highlight all index by
  // `orderedEntries`. "" (Playlist order) keeps the manual entry order.
  const orderedEntries =
    sort === ""
      ? renderEntries
      : [...renderEntries].sort((a, b) => {
          const ta = a.track!;
          const tb = b.track!;
          if (sort === "track") {
            const disc = (ta.disc_number ?? 0) - (tb.disc_number ?? 0);
            return disc !== 0 ? disc : (ta.track_number ?? 0) - (tb.track_number ?? 0);
          }
          const keyA =
            sort === "title"
              ? trackTitle(ta)
              : sort === "artist"
                ? trackArtist(ta)
                : trackAlbum(ta);
          const keyB =
            sort === "title"
              ? trackTitle(tb)
              : sort === "artist"
                ? trackArtist(tb)
                : trackAlbum(tb);
          return keyA.localeCompare(keyB);
        });

  const unavailableEntries = entries.filter(
    (entry) =>
      !entry.track &&
      (!entrySearch ||
        entry.saved_title?.toLocaleLowerCase().includes(entrySearch.toLocaleLowerCase())),
  );
  useEffect(() => {
    setOffset((previous) =>
      Math.min(previous, Math.max(0, Math.ceil(orderedEntries.length / 50) - 1) * 50),
    );
  }, [orderedEntries.length]);
  const pageEntries = orderedEntries.slice(offset, offset + 50);
  const pageTracks = pageEntries.map((entry) => entry.track!);
  useEffect(() => setOffset(0), [playlist?.playlist_id, sort, entrySearch]);

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

  const removeTrack = async (entryId: string) => {
    await removeEntry.mutateAsync({
      params: {
        path: { id: playlist.playlist_id ?? "", entryId },
        query: { revision: playlist.revision },
      },
    });
    await invalidateMusic();
  };

  const handleReorder = async (activeId: string, overId: string) => {
    const from = entries.findIndex((entry) => entry.entry_id === activeId);
    const to = entries.findIndex((entry) => entry.entry_id === overId);
    if (from === -1 || to === -1 || from === to) return;
    const reordered = [...entries];
    const [moved] = reordered.splice(from, 1);
    if (!moved) return;
    reordered.splice(to, 0, moved);
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
      <MusicDetailToolbar
        editLabel={t("music.playlist.edit", "Edit playlist")}
        onEdit={() => setEditing(!editing)}
        moreActions={[
          {
            label: t("music.playlist.delete", "Delete playlist"),
            onSelect: () => {
              void run(removePlaylist);
            },
            danger: true,
          },
        ]}
        sort={sort}
        onSortChange={setSort}
        sortEmptyLabel={t("music.playlist.entryHeading", "Playlist order")}
        search={{
          value: entrySearch,
          onChange: setEntrySearch,
          placeholder: t("music.searchTracks", "Search tracks"),
          ariaLabel: t("music.searchTracks", "Search tracks"),
        }}
      />
      <div className="music-detail-hero playlist">
        <div className="music-detail-placeholder">
          <MusicArtwork assetId={playlist.cover_asset_id} alt={playlist.title ?? ""} size="cover" />
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
              type="button"
              className="btn btn-ghost btn-circle"
              aria-label={t("music.player.shuffle", "Shuffle")}
              aria-pressed={shuffle}
              onClick={() => {
                if (!shuffle) toggleShuffle();
                void playSource({ kind: "playlist", id: playlist.playlist_id });
              }}
            >
              <Shuffle className="size-4" />
            </button>
            <button
              className="btn btn-primary"
              disabled={!entries.length}
              onClick={() => void playSource({ kind: "playlist", id: playlist.playlist_id })}
            >
              <Play className="size-4" fill="currentColor" />
              {t("music.player.play", "Play")}
            </button>
          </div>
        </div>
      </div>
      <div className="w-full">
        <div className="w-full space-y-6">
          <Modal
            open={editing}
            onClose={() => setEditing(false)}
            title={t("music.playlist.edit", "Edit playlist")}
            icon={<ListMusic className="size-5" />}
            size="sm"
            footer={
              <>
                <button type="button" className="btn btn-ghost" onClick={() => setEditing(false)}>
                  {t("common.cancel", "Cancel")}
                </button>
                <button
                  type="submit"
                  form="edit-playlist-form"
                  className="btn btn-primary"
                  disabled={updatePlaylist.isPending}
                >
                  <Save className="size-4" />
                  {t("music.actions.save", "Save changes")}
                </button>
              </>
            }
          >
            <form
              id="edit-playlist-form"
              onSubmit={(event) => {
                event.preventDefault();
                void run(() => save(event));
              }}
              className="grid gap-4 p-4 sm:p-6"
            >
              <label className="grid gap-2">
                <span className="text-sm font-medium text-base-content/70">
                  {t("music.fields.playlistTitle", "Playlist title")}
                </span>
                <input
                  className="input input-bordered w-full"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  required
                />
              </label>
              <label className="grid gap-2">
                <span className="text-sm font-medium text-base-content/70">
                  {t("music.fields.description", "Description")}
                </span>
                <textarea
                  className="textarea textarea-bordered w-full min-h-24"
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                />
              </label>
            </form>
          </Modal>
          <section>
            {unavailableEntries.map((entry) => (
              <div
                key={entry.entry_id}
                className="flex items-center justify-between gap-3 p-3 text-sm opacity-60"
              >
                <span>
                  {entry.saved_title} · {t("music.queue.unavailable", "Unavailable")}
                </span>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  disabled={removeEntry.isPending}
                  onClick={() => void run(() => removeTrack(entry.entry_id ?? ""))}
                >
                  {t("music.playlist.remove", "Remove entry")}
                </button>
              </div>
            ))}
            {renderEntries.length === 0 && unavailableEntries.length === 0 ? (
              <p className="p-5 text-sm text-base-content/60">
                {entrySearch
                  ? t("music.search.empty", "No matching tracks")
                  : t("music.playlist.empty", "This playlist has no entries yet.")}
              </p>
            ) : (
              <>
                <MusicTrackList
                  tracks={pageTracks}
                  source={{ kind: "playlist", id: playlist.playlist_id ?? "" }}
                  getEntryId={(_, i) => pageEntries[i].entry_id}
                  onRemove={(i) => void run(() => removeTrack(pageEntries[i].entry_id ?? ""))}
                  sortable={
                    sort === "" && !entrySearch
                      ? {
                          getKey: (_, i) => pageEntries[i].entry_id ?? String(i),
                          onReorder: (activeId, overId) =>
                            void run(() => handleReorder(activeId, overId)),
                        }
                      : undefined
                  }
                />
                {orderedEntries.length > 50 && (
                  <MusicPagination
                    offset={offset}
                    pageSize={50}
                    total={orderedEntries.length}
                    onPrevious={() => setOffset(Math.max(0, offset - 50))}
                    onNext={() => setOffset(offset + 50)}
                  />
                )}
              </>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

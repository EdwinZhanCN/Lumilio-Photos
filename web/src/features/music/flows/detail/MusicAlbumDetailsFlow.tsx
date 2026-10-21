import { useMusicFeedback } from "../../state/useMusicFeedback";
import { FormEvent, useEffect, useState } from "react";
import { Disc3, Play, Save, Shuffle } from "lucide-react";
import { useParams } from "react-router-dom";

import { useI18n } from "@/lib/i18n";
import Modal from "@/components/ui/Modal";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMusicAlbum, useMusicMutations } from "../../api/useMusic";
import MusicArtwork from "../../components/MusicArtwork";
import MusicTrackList from "../../components/MusicTrackList";
import MusicPagination from "../../components/MusicPagination";
import MusicDetailToolbar from "../../components/MusicDetailToolbar";
import type { MusicSortValue } from "../../components/MusicSortDropdown";
import { sortMusicTracks, trackAlbum, trackArtist, trackTitle } from "../../model/music";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";

export default function MusicAlbumDetailsFlow() {
  const { t } = useI18n();
  const { run, feedback } = useMusicFeedback();
  const { albumId } = useParams<{ albumId: string }>();
  const albumQuery = useMusicAlbum(albumId);
  const { playSource, shuffle, toggleShuffle } = useMusicPlayer();
  const { updateAlbum, invalidateMusic } = useMusicMutations();
  const [editing, setEditing] = useState(false);
  const [sort, setSort] = useState<MusicSortValue>("");
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [title, setTitle] = useState("");
  const album = albumQuery.data;
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("music.title"), to: "/music" },
    { label: t("music.views.albums", "Albums"), to: "/music?view=albums" },
    { label: album?.title || t("music.views.albums", "Albums") },
  ]);
  const allTracks = album?.tracks ?? [];
  const filteredTracks = allTracks.filter((track) => {
    if (!search) return true;
    const text = [trackTitle(track), trackArtist(track), trackAlbum(track)].join(" ").toLowerCase();
    return text.includes(search.toLowerCase());
  });
  const sortedTracks = sortMusicTracks(filteredTracks, sort);
  const pageTracks = sortedTracks.slice(offset, offset + 50);
  const totalTracks = sortedTracks.length;

  useEffect(() => {
    if (album) setTitle(album.title ?? "");
  }, [album]);
  useEffect(() => setOffset(0), [album?.album_id, sort, search]);

  if (albumQuery.isPending)
    return (
      <div className="flex h-full items-center justify-center">
        <span className="loading loading-spinner loading-md text-primary" />
      </div>
    );
  if (albumQuery.isError || !album)
    return (
      <div className="p-6">
        <div className="alert alert-error">
          {t("music.detail.error", "This music item could not be loaded.")}
        </div>
      </div>
    );

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await updateAlbum.mutateAsync({
      params: { path: { id: album.album_id ?? "" } },
      body: { title, revision: album.revision },
    });
    await invalidateMusic();
    setEditing(false);
  };

  return (
    <div className="music-browse music-scroll">
      {feedback}
      <MusicDetailToolbar
        editLabel={t("music.actions.editAlbum", "Edit album")}
        onEdit={() => setEditing(!editing)}
        sort={sort}
        onSortChange={setSort}
        favorite={{
          kind: "album",
          id: album.album_id,
          favorite: album.favorite,
          revision: album.revision,
        }}
        search={{
          value: search,
          onChange: setSearch,
          placeholder: t("music.searchTracks", "Search tracks"),
          ariaLabel: t("music.searchTracks", "Search tracks"),
        }}
      />
      <div className="w-full">
        <div className="w-full space-y-8">
          <div className="music-detail-hero">
            <MusicArtwork
              assetId={album.cover_asset_id}
              alt={album.title ?? t("music.album.untitled", "Untitled album")}
              size="cover"
            />
            <div className="space-y-4">
              <div>
                <div className="flex items-center gap-2 text-primary">
                  <Disc3 className="size-4" />
                  <span className="text-sm font-medium">
                    {t("music.album.label", "Music Album")}
                  </span>
                </div>
                <h1 className="mt-3 text-3xl font-bold tracking-tight sm:text-4xl">
                  {album.title}
                </h1>
                <p className="mt-3 text-base text-base-content/70">
                  {album.artists?.map((artist) => artist.display_name).join(" · ") ||
                    t("music.unknownArtist", "Unknown artist")}
                </p>
                <p className="mt-1 text-xs text-base-content/50">
                  {album.release_date || t("music.album.noDate", "Release date unknown")}
                  {album.edition ? ` · ${album.edition}` : ""}
                </p>
              </div>
              <div className="music-detail-actions">
                <button
                  type="button"
                  className="btn btn-ghost btn-circle"
                  aria-label={t("music.player.shuffle", "Shuffle")}
                  aria-pressed={shuffle}
                  onClick={() => {
                    if (!shuffle) toggleShuffle();
                    void playSource({ kind: "album", id: album.album_id ?? "" });
                  }}
                >
                  <Shuffle className="size-4" />
                </button>
                <button
                  type="button"
                  className="btn btn-sm btn-primary"
                  onClick={() => void playSource({ kind: "album", id: album.album_id ?? "" })}
                >
                  <Play className="size-4" fill="currentColor" />
                  {t("music.actions.playAlbum", "Play album")}
                </button>
              </div>
              <Modal
                open={editing}
                onClose={() => setEditing(false)}
                title={t("music.actions.editAlbum", "Edit album")}
                icon={<Disc3 className="size-5" />}
                size="sm"
                footer={
                  <>
                    <button
                      type="button"
                      className="btn btn-ghost"
                      onClick={() => setEditing(false)}
                    >
                      {t("common.cancel", "Cancel")}
                    </button>
                    <button
                      type="submit"
                      form="edit-album-form"
                      className="btn btn-primary"
                      disabled={updateAlbum.isPending}
                    >
                      <Save className="size-4" />
                      {t("music.actions.save", "Save changes")}
                    </button>
                  </>
                }
              >
                <form
                  id="edit-album-form"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void run(() => save(event));
                  }}
                  className="grid gap-4 p-4 sm:p-6"
                >
                  <label className="grid gap-2">
                    <span className="text-sm font-medium text-base-content/70">
                      {t("music.fields.album", "Album")}
                    </span>
                    <input
                      className="input input-bordered w-full"
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      required
                    />
                  </label>
                </form>
              </Modal>
            </div>
          </div>
          <section className="py-2">
            {totalTracks === 0 ? (
              <p className="p-5 text-sm text-base-content/60">
                {t("music.album.emptyTracks", "No available tracks in this release.")}
              </p>
            ) : (
              <>
                <MusicTrackList
                  tracks={pageTracks}
                  source={{ kind: "album", id: album.album_id ?? "" }}
                  showAlbumColumn={false}
                />
                {totalTracks > 50 && (
                  <MusicPagination
                    offset={offset}
                    pageSize={50}
                    total={totalTracks}
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

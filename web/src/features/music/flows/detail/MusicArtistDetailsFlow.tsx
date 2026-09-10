import { useMusicFeedback } from "../../state/useMusicFeedback";
import { FormEvent, useEffect, useState } from "react";
import { Mic2, Save, Play, Shuffle } from "lucide-react";
import { useParams } from "react-router-dom";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";
import { useI18n } from "@/lib/i18n";
import Modal from "@/components/ui/Modal";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMusicArtist, useMusicMutations, useMusicTracks } from "../../api/useMusic";
import MusicTrackList from "../../components/MusicTrackList";
import MusicPagination from "../../components/MusicPagination";
import MusicDetailToolbar from "../../components/MusicDetailToolbar";
import type { MusicSortValue } from "../../components/MusicSortDropdown";

export default function MusicArtistDetailsFlow() {
  const { t } = useI18n();
  const { playSource, shuffle, toggleShuffle } = useMusicPlayer();
  const [editing, setEditing] = useState(false);
  const [sort, setSort] = useState<MusicSortValue>("");
  const [search, setSearch] = useState("");
  const { run, feedback } = useMusicFeedback();
  const { artistId } = useParams<{ artistId: string }>();
  const artistQuery = useMusicArtist(artistId);
  const artist = artistQuery.data;
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("music.title"), to: "/music" },
    { label: t("music.views.artists", "Artists"), to: "/music?view=artists" },
    { label: artist?.display_name || t("music.views.artists", "Artists") },
  ]);
  const [offset, setOffset] = useState(0);
  useEffect(() => setOffset(0), [sort, artistId, search]);
  const tracksQuery = useMusicTracks({
    artistId,
    sort,
    query: search || undefined,
    limit: 50,
    offset,
    enabled: Boolean(artistId),
  });
  const { updateArtist, invalidateMusic } = useMusicMutations();
  const [name, setName] = useState("");

  useEffect(() => {
    if (artist) setName(artist.display_name ?? "");
  }, [artist]);

  if (artistQuery.isPending)
    return (
      <div className="flex h-full items-center justify-center">
        <span className="loading loading-spinner loading-md text-primary" />
      </div>
    );
  if (artistQuery.isError || !artist)
    return (
      <div className="p-6">
        <div className="alert alert-error">
          {t("music.detail.error", "This music item could not be loaded.")}
        </div>
      </div>
    );

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await updateArtist.mutateAsync({
      params: { path: { id: artist.artist_id ?? "" } },
      body: { display_name: name, revision: artist.revision },
    });
    await invalidateMusic();
    setEditing(false);
  };

  return (
    <div className="music-browse music-scroll">
      {feedback}
      <MusicDetailToolbar
        editLabel={t("music.actions.editArtist", "Edit artist")}
        onEdit={() => setEditing(!editing)}
        sort={sort}
        onSortChange={setSort}
        favorite={{
          kind: "artist",
          id: artist.artist_id,
          favorite: artist.favorite,
          revision: artist.revision,
        }}
        search={{
          value: search,
          onChange: setSearch,
          placeholder: t("music.searchTracks", "Search tracks"),
          ariaLabel: t("music.searchTracks", "Search tracks"),
        }}
      />
      <div className="music-detail-hero">
        <div className="music-detail-placeholder !rounded-full">
          <Mic2 className="size-24 opacity-30" />
        </div>
        <div>
          <h1>{artist.display_name}</h1>
          <p className="opacity-60">
            {artist.track_count ?? 0} {t("music.tracks.count", "tracks")} ·{" "}
            {artist.album_count ?? 0} {t("music.artist.albums", "albums")}
          </p>
          <div className="music-detail-actions">
            <button
              type="button"
              className="btn btn-ghost btn-circle"
              aria-label={t("music.player.shuffle", "Shuffle")}
              aria-pressed={shuffle}
              onClick={() => {
                if (!shuffle) toggleShuffle();
                void playSource({
                  kind: "query",
                  artist_id: artistId,
                  query: search,
                  sort: sort || undefined,
                });
              }}
            >
              <Shuffle className="size-4" />
            </button>
            <button
              className="btn btn-primary"
              onClick={() =>
                void playSource({
                  kind: "query",
                  artist_id: artistId,
                  query: search,
                  sort: sort || undefined,
                })
              }
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
            title={t("music.actions.editArtist", "Edit artist")}
            icon={<Mic2 className="size-5" />}
            size="sm"
            footer={
              <>
                <button type="button" className="btn btn-ghost" onClick={() => setEditing(false)}>
                  {t("common.cancel", "Cancel")}
                </button>
                <button
                  type="submit"
                  form="edit-artist-form"
                  className="btn btn-primary"
                  disabled={updateArtist.isPending}
                >
                  <Save className="size-4" />
                  {t("music.actions.save", "Save changes")}
                </button>
              </>
            }
          >
            <form
              id="edit-artist-form"
              onSubmit={(event) => {
                event.preventDefault();
                void run(() => save(event));
              }}
              className="grid gap-4 p-4 sm:p-6"
            >
              <label className="grid gap-2">
                <span className="text-sm font-medium text-base-content/70">
                  {t("music.fields.artist", "Artist")}
                </span>
                <input
                  className="input input-bordered w-full"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  required
                />
              </label>
            </form>
          </Modal>
          <section className="space-y-4">
            {tracksQuery.isPending ? (
              <span className="loading loading-spinner" />
            ) : tracksQuery.isError ? (
              <button className="btn btn-error" onClick={() => void tracksQuery.refetch()}>
                {t("common.retry", "Retry")}
              </button>
            ) : (tracksQuery.data?.items ?? []).length === 0 ? (
              <p className="p-5 text-sm text-base-content/60">
                {t("music.artist.emptyTracks", "No tracks found for this artist.")}
              </p>
            ) : (
              <MusicTrackList
                tracks={tracksQuery.data?.items ?? []}
                source={{
                  kind: "query",
                  artist_id: artistId,
                  query: search,
                  sort: sort || undefined,
                }}
                indexOffset={offset}
              />
            )}
            {(tracksQuery.data?.total ?? 0) > 50 && (
              <MusicPagination
                offset={offset}
                pageSize={50}
                total={tracksQuery.data?.total ?? 0}
                isFetching={tracksQuery.isFetching}
                onPrevious={() => setOffset(Math.max(0, offset - 50))}
                onNext={() => setOffset(offset + 50)}
              />
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

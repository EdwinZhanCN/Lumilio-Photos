import MusicMoreMenu from "../../components/MusicMoreMenu";
import MusicFavoriteButton from "../../components/MusicFavoriteButton";

import { useMusicFeedback } from "../../state/useMusicFeedback";
import { FormEvent, useEffect, useState } from "react";
import { Mic2, Save, Play } from "lucide-react";
import { useParams } from "react-router-dom";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";
import { useI18n } from "@/lib/i18n";
import { useMusicArtist, useMusicMutations, useMusicTracks } from "../../api/useMusic";
import MusicTrackRow from "../../components/MusicTrackRow";

export default function MusicArtistDetailsFlow() {
  const { t } = useI18n();
  const { playSource } = useMusicPlayer();
  const [editing, setEditing] = useState(false);
  const { run, feedback } = useMusicFeedback();
  const { artistId } = useParams<{ artistId: string }>();
  const artistQuery = useMusicArtist(artistId);
  const artist = artistQuery.data;
  const [offset, setOffset] = useState(0);
  const tracksQuery = useMusicTracks({
    artistId,
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
              className="btn btn-primary"
              onClick={() => void playSource({ kind: "query", artist_id: artistId })}
            >
              <Play className="size-4" fill="currentColor" />
              {t("music.player.play", "Play")}
            </button>
            <MusicFavoriteButton
              kind="artist"
              id={artist.artist_id}
              favorite={artist.favorite}
              revision={artist.revision}
            />
            <MusicMoreMenu
              actions={[
                {
                  label: t("music.actions.editArtist", "Edit artist"),
                  onSelect: () => setEditing(!editing),
                },
              ]}
            />
          </div>
        </div>
      </div>
      <div className="w-full">
        <div className="w-full space-y-6">
          {editing && (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void run(() => save(event));
              }}
              className="flex flex-col gap-2 rounded-2xl border border-base-300/70 bg-base-200/35 p-4 sm:flex-row sm:items-end"
            >
              <label className="form-control flex-1">
                <span className="label-text text-xs">{t("music.fields.artist", "Artist")}</span>
                <input
                  className="input input-bordered input-sm"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </label>
              <button
                className="btn btn-sm btn-secondary"
                type="submit"
                disabled={updateArtist.isPending}
              >
                <Save className="size-4" />
                {t("music.actions.save", "Save changes")}
              </button>
            </form>
          )}
          <section className="space-y-4">
            <h2 className="mb-2 text-lg font-semibold">
              {t("music.artist.tracksHeading", "Tracks credited to this artist")}
            </h2>
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
              <div className="space-y-1">
                {(tracksQuery.data?.items ?? []).map((track, index) => (
                  <MusicTrackRow
                    key={track.track_id ?? index}
                    track={track}
                    index={offset + index}
                    source={{ kind: "query", artist_id: artistId }}
                  />
                ))}
              </div>
            )}
            <div className="mt-4 flex justify-between">
              <button
                className="btn btn-sm"
                disabled={offset === 0 || tracksQuery.isFetching}
                onClick={() => setOffset(Math.max(0, offset - 50))}
              >
                {t("common.previous", "Previous")}
              </button>
              <button
                className="btn btn-sm"
                disabled={offset + 50 >= (tracksQuery.data?.total ?? 0) || tracksQuery.isFetching}
                onClick={() => setOffset(offset + 50)}
              >
                {t("common.next", "Next")}
              </button>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

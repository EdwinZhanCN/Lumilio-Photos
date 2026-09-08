import MusicLyrics from "../../components/MusicLyrics";
import { $api } from "@/lib/http-commons/queryClient";
import MusicPlaylistAction from "../../components/MusicPlaylistAction";
import Link from "../../components/MusicLink";
import { useMusicFeedback } from "../../state/useMusicFeedback";
import { useEffect, useState, type FormEvent } from "react";
import { ArrowLeft, Play, RotateCcw, Save } from "lucide-react";
import { useParams } from "react-router-dom";
import PageHeader from "@/components/ui/PageHeader";
import { useI18n } from "@/lib/i18n";
import { useMusicMutations, useMusicTrack } from "../../api/useMusic";
import MusicArtwork from "../../components/MusicArtwork";
import { formatMusicDuration, trackArtist, trackTitle } from "../../model/music";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";
import MusicLikeButton from "../../components/MusicLikeButton";

export default function MusicTrackDetailsFlow() {
  const { t } = useI18n();
  const { run, feedback } = useMusicFeedback();
  const { trackId } = useParams<{ trackId: string }>();
  const trackQuery = useMusicTrack(trackId);
  const regenerate = $api.useMutation("post", "/api/v1/assets/{id}/reprocess");
  const { playTrack } = useMusicPlayer();
  const { updateTrack, resetTrack, invalidateMusic } = useMusicMutations();
  const [draft, setDraft] = useState({
    title: "",
    artist: "",
    album: "",
    albumArtist: "",
    genre: "",
    releaseDate: "",
    designation: "music",
  });

  const track = trackQuery.data;
  useEffect(() => {
    if (!track) return;
    setDraft({
      title: track.title ?? "",
      artist: track.artist_name ?? "",
      album: track.album_title ?? "",
      albumArtist: track.album_artist_name ?? "",
      genre: track.genre ?? "",
      releaseDate: track.release_date ?? "",
      designation: track.designation ?? "music",
    });
  }, [track]);

  if (trackQuery.isPending) return <DetailLoading />;
  if (trackQuery.isError || !track) return <DetailError />;

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await updateTrack.mutateAsync({
      params: { path: { id: track.track_id ?? "" } },
      body: {
        title: draft.title,
        artist_name: draft.artist,
        album_title: draft.album,
        album_artist_name: draft.albumArtist,
        genre: draft.genre,
        release_date: draft.releaseDate,
        designation: draft.designation,
        revision: track.revision,
      },
    });
    await invalidateMusic();
  };

  const reset = async () => {
    if (
      !window.confirm(
        t("music.track.confirmReset", "Reset all corrections to the original file metadata?"),
      )
    )
      return;
    await resetTrack.mutateAsync({ params: { path: { id: track.track_id ?? "" } } });
    await invalidateMusic();
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-base-100">
      {feedback}
      <PageHeader
        title={trackTitle(track)}
        contentClassName="mx-auto w-full max-w-5xl px-4 py-3 sm:px-6 lg:px-8"
        icon={
          <Link
            to="/music"
            className="btn btn-ghost btn-circle btn-sm"
            aria-label={t("music.actions.back", "Back to music")}
          >
            <ArrowLeft className="size-5" />
          </Link>
        }
        subtitle={trackArtist(track)}
      >
        <MusicLikeButton trackId={track.track_id} track={track} />
        <button
          type="button"
          className="btn btn-sm btn-primary"
          onClick={() => void playTrack(track)}
        >
          <Play className="size-4" fill="currentColor" />
          {t("music.actions.play", "Play")}
        </button>
      </PageHeader>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-8 sm:px-6 lg:px-8">
        <div className="mx-auto grid w-full max-w-5xl gap-6 py-6 lg:grid-cols-[16rem_minmax(0,1fr)]">
          <div className="space-y-3">
            <MusicArtwork assetId={track.track_id} alt={trackTitle(track)} size="lg" />
            <button
              className="btn btn-ghost btn-sm"
              disabled={regenerate.isPending}
              onClick={() =>
                void run(async () => {
                  await regenerate.mutateAsync({
                    params: { path: { id: track.track_id ?? "" } },
                    body: { tasks: ["derivatives"] },
                  });
                })
              }
            >
              {t("music.track.regenerateArtwork", "Regenerate artwork")}
            </button>
            {regenerate.isSuccess && (
              <p role="status" className="text-sm text-base-content/60">
                {t(
                  "music.track.artworkQueued",
                  "Artwork regeneration queued. Refresh after processing finishes.",
                )}
              </p>
            )}
            <div className="rounded-2xl border border-base-300/70 bg-base-200/35 p-4 text-sm">
              <p className="font-medium">{t("music.track.file", "File")}</p>
              <p className="mt-1 break-all text-base-content/60">{track.original_filename}</p>
              <dl className="mt-3 space-y-1 text-xs text-base-content/60">
                <div className="flex justify-between gap-3">
                  <dt>{t("music.track.duration", "Duration")}</dt>
                  <dd>{formatMusicDuration(track.duration)}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt>{t("music.track.format", "Format")}</dt>
                  <dd>{track.mime_type || "—"}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt>{t("music.track.designation", "Designation")}</dt>
                  <dd>{track.designation}</dd>
                </div>
              </dl>
            </div>
          </div>
          <div className="space-y-4">
            <form
              onSubmit={(event) => {
                event.preventDefault();
                void run(() => save(event));
              }}
              className="rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-sm sm:p-5"
            >
              <div className="mb-4 flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold">
                    {t("music.track.editTitle", "Correct metadata")}
                  </h2>
                  <p className="text-sm text-base-content/60">
                    {t(
                      "music.track.editHint",
                      "Changes are stored as overrides and never write to the original file.",
                    )}
                  </p>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => void run(() => reset())}
                  disabled={resetTrack.isPending || track.overrides?.length === 0}
                >
                  <RotateCcw className="size-4" />
                  {t("music.actions.reset", "Reset")}
                </button>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="form-control sm:col-span-2">
                  <span className="label-text text-xs">{t("music.fields.title", "Title")}</span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.title}
                    onChange={(event) => setDraft({ ...draft, title: event.target.value })}
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">
                    {t("music.fields.artist", "Track artist")}
                  </span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.artist}
                    onChange={(event) => setDraft({ ...draft, artist: event.target.value })}
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">
                    {t("music.fields.albumArtist", "Album artist")}
                  </span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.albumArtist}
                    onChange={(event) => setDraft({ ...draft, albumArtist: event.target.value })}
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">{t("music.fields.album", "Album")}</span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.album}
                    onChange={(event) => setDraft({ ...draft, album: event.target.value })}
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">{t("music.fields.genre", "Genre")}</span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.genre}
                    onChange={(event) => setDraft({ ...draft, genre: event.target.value })}
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">
                    {t("music.fields.releaseDate", "Release date")}
                  </span>
                  <input
                    className="input input-bordered input-sm"
                    value={draft.releaseDate}
                    onChange={(event) => setDraft({ ...draft, releaseDate: event.target.value })}
                    placeholder="YYYY-MM-DD"
                  />
                </label>
                <label className="form-control">
                  <span className="label-text text-xs">
                    {t("music.fields.designation", "Show in")}
                  </span>
                  <select
                    className="select select-bordered select-sm"
                    value={draft.designation}
                    onChange={(event) => setDraft({ ...draft, designation: event.target.value })}
                  >
                    <option value="music">{t("music.designation.music", "Music")}</option>
                    <option value="other">{t("music.designation.other", "Other audio")}</option>
                  </select>
                </label>
              </div>
              <div className="mt-4 flex justify-end">
                <button
                  type="submit"
                  className="btn btn-sm btn-primary"
                  disabled={updateTrack.isPending}
                >
                  <Save className="size-4" />
                  {t("music.actions.save", "Save changes")}
                </button>
              </div>
            </form>
            <MusicPlaylistAction trackId={track.track_id} />
            <MusicLyrics trackId={track.track_id} editable />
          </div>
        </div>
      </div>
    </div>
  );
}

function DetailLoading() {
  return (
    <div className="flex h-full items-center justify-center">
      <span className="loading loading-spinner loading-md text-primary" />
    </div>
  );
}

function DetailError() {
  const { t } = useI18n();
  return (
    <div className="p-6">
      <div className="alert alert-error">
        {t("music.detail.error", "This music item could not be loaded.")}
      </div>
      <Link className="btn btn-ghost mt-4" to="/music">
        {t("music.actions.back", "Back to music")}
      </Link>
    </div>
  );
}

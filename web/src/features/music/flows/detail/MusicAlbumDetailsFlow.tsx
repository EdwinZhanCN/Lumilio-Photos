import MusicMoreMenu from "../../components/MusicMoreMenu";
import MusicFavoriteButton from "../../components/MusicFavoriteButton";

import { useMusicFeedback } from "../../state/useMusicFeedback";
import { FormEvent, useEffect, useState } from "react";
import { Disc3, Play, Save } from "lucide-react";
import { useParams } from "react-router-dom";

import { useI18n } from "@/lib/i18n";
import { useMusicAlbum, useMusicMutations } from "../../api/useMusic";
import MusicArtwork from "../../components/MusicArtwork";
import MusicTrackRow from "../../components/MusicTrackRow";
import { useMusicPlayer } from "../../state/MusicPlayerProvider";

export default function MusicAlbumDetailsFlow() {
  const { t } = useI18n();
  const { run, feedback } = useMusicFeedback();
  const { albumId } = useParams<{ albumId: string }>();
  const albumQuery = useMusicAlbum(albumId);
  const { playSource } = useMusicPlayer();
  const { updateAlbum, invalidateMusic } = useMusicMutations();
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState("");
  const album = albumQuery.data;

  useEffect(() => {
    if (album) setTitle(album.title ?? "");
  }, [album]);

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
                  className="btn btn-sm btn-primary"
                  onClick={() => void playSource({ kind: "album", id: album.album_id ?? "" })}
                >
                  <Play className="size-4" fill="currentColor" />
                  {t("music.actions.playAlbum", "Play album")}
                </button>
                <MusicFavoriteButton
                  kind="album"
                  id={album.album_id}
                  favorite={album.favorite}
                  revision={album.revision}
                />
                <MusicMoreMenu
                  actions={[
                    {
                      label: t("music.actions.editAlbum", "Edit album"),
                      onSelect: () => setEditing(!editing),
                    },
                  ]}
                />
              </div>
              {editing && (
                <form
                  onSubmit={(event) => {
                    event.preventDefault();
                    void run(() => save(event));
                  }}
                  className="flex flex-col gap-2 sm:flex-row"
                >
                  <input
                    className="input input-bordered input-sm flex-1"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    aria-label={t("music.fields.album", "Album")}
                  />
                  <button
                    className="btn btn-sm btn-secondary"
                    type="submit"
                    disabled={updateAlbum.isPending}
                  >
                    <Save className="size-4" />
                    {t("music.actions.save", "Save changes")}
                  </button>
                </form>
              )}
            </div>
          </div>
          <section className="py-2">
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-lg font-semibold">{t("music.album.trackList", "Track list")}</h2>
              <span className="text-xs text-base-content/50">{album.tracks?.length ?? 0}</span>
            </div>
            {(album.tracks ?? []).length === 0 ? (
              <p className="p-5 text-sm text-base-content/60">
                {t("music.album.emptyTracks", "No available tracks in this release.")}
              </p>
            ) : (
              <div className="space-y-1">
                {(album.tracks ?? []).map((track, index) => (
                  <MusicTrackRow
                    key={track.track_id ?? index}
                    track={track}
                    index={index}
                    source={{ kind: "album", id: album.album_id ?? "" }}
                  />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

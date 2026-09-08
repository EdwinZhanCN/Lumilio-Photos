import type { MusicPlaylist } from "../model/music";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ListMusic, Plus, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useMusicMutations, useMusicPlaylists } from "../api/useMusic";
import { useMusicFeedback } from "../state/useMusicFeedback";

/** Shared create/add flow; native dialog owns focus, Escape and focus restoration. */
export default function MusicPlaylistAction({
  trackId,
  showLabel = false,
}: {
  trackId?: string;
  showLabel?: boolean;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const [offset, setOffset] = useState(0);
  const [title, setTitle] = useState("");
  const [creating, setCreating] = useState(!trackId);
  const createdPlaylist = useRef<MusicPlaylist | undefined>(undefined);
  const dialog = useRef<HTMLDialogElement>(null);
  const playlists = useMusicPlaylists({ enabled: open && Boolean(trackId), limit: 20, offset });
  const { createPlaylist, addEntry, invalidateMusic } = useMusicMutations();
  const { run, feedback, pending } = useMusicFeedback();
  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);
  const add = async (id: string, revision?: number) => {
    if (!trackId) return;
    await addEntry.mutateAsync({
      params: { path: { id } },
      body: {
        track_id: trackId,
        revision,
        idempotency_key: crypto.randomUUID(),
      },
    });
    await invalidateMusic();
  };
  const label = trackId
    ? t("music.playlists.addLabel", "Add to playlist")
    : t("music.playlists.create", "Create playlist");
  return (
    <>
      <button
        type="button"
        className="btn btn-ghost btn-sm"
        aria-label={label}
        onClick={() => {
          createdPlaylist.current = undefined;
          setCreating(!trackId);
          setTitle("");
          setOffset(0);
          setOpen(true);
        }}
      >
        <ListMusic className="size-4" />
        {(!trackId || showLabel) && label}
      </button>
      {createPortal(
        <dialog
          ref={dialog}
          className="modal music-modal"
          onClose={() => setOpen(false)}
          onClick={(event) => {
            if (event.target === event.currentTarget && !pending) setOpen(false);
          }}
          onCancel={(event) => {
            if (pending) event.preventDefault();
          }}
        >
          <div className="modal-box max-w-md">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-bold">
                {creating ? t("music.playlists.create", "Create playlist") : label}
              </h2>
              <button
                type="button"
                className="btn btn-ghost btn-circle btn-sm"
                disabled={pending}
                onClick={() => setOpen(false)}
                aria-label={t("common.close", "Close")}
              >
                <X className="size-4" />
              </button>
            </div>
            {feedback}
            {creating && (
              <form
                className="space-y-4"
                onSubmit={(event) => {
                  event.preventDefault();
                  void run(async () => {
                    const playlist =
                      createdPlaylist.current ??
                      (await createPlaylist.mutateAsync({
                        body: { title: title.trim() },
                      }));
                    createdPlaylist.current = playlist;
                    await invalidateMusic();
                    if (playlist?.playlist_id && trackId)
                      await add(playlist.playlist_id, playlist.revision);
                    setTitle("");
                    setOpen(false);
                  });
                }}
              >
                <label className="block space-y-2">
                  <span className="text-sm font-medium">
                    {t("music.fields.playlistTitle", "Playlist title")}
                  </span>
                  <input
                    className="input input-bordered w-full"
                    required
                    maxLength={40}
                    disabled={pending || Boolean(createdPlaylist.current)}
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    aria-label={t("music.fields.playlistTitle", "Playlist title")}
                  />
                </label>
                <button className="btn btn-primary w-full" disabled={pending || !title.trim()}>
                  {t("music.playlists.create", "Create playlist")}
                </button>
              </form>
            )}
            {trackId && !creating && (
              <div className="mt-4 space-y-2">
                <button
                  className="btn btn-ghost w-full justify-start"
                  onClick={() => setCreating(true)}
                >
                  <Plus className="size-5" />
                  {t("music.playlists.create", "Create playlist")}
                </button>
                {playlists.isPending && <span className="loading loading-spinner" />}
                {playlists.isError && (
                  <button className="btn btn-error" onClick={() => void playlists.refetch()}>
                    {t("common.retry", "Retry")}
                  </button>
                )}
                {(playlists.data?.items ?? []).map((playlist) => (
                  <button
                    key={playlist.playlist_id}
                    className="btn btn-ghost w-full justify-start truncate"
                    disabled={pending}
                    onClick={() =>
                      void run(async () => {
                        await add(playlist.playlist_id ?? "", playlist.revision);
                        setOpen(false);
                      })
                    }
                  >
                    <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-base-200">
                      <ListMusic className="size-5" />
                    </span>
                    <span className="min-w-0 text-left">
                      <span className="block truncate">{playlist.title}</span>
                      <span className="block text-xs font-normal opacity-50">
                        {playlist.entry_count ?? 0} {t("music.tracks.count", "tracks")}
                      </span>
                    </span>
                  </button>
                ))}
                <div className="flex justify-between">
                  <button
                    className="btn btn-sm"
                    disabled={offset === 0 || pending}
                    onClick={() => setOffset(Math.max(0, offset - 20))}
                  >
                    {t("common.previous", "Previous")}
                  </button>
                  <button
                    className="btn btn-sm"
                    disabled={pending || (playlists.data?.items?.length ?? 0) < 20}
                    onClick={() => setOffset(offset + 20)}
                  >
                    {t("common.next", "Next")}
                  </button>
                </div>
              </div>
            )}
          </div>
        </dialog>,
        document.body,
      )}
    </>
  );
}

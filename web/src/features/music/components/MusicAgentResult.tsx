import { useState } from "react";
import { ArrowDown, ArrowUp, Play } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useMusicPlayer } from "../state/MusicPlayerProvider";
import { trackId, trackTitle, trackArtist } from "../model/music";

type Props = {
  refId: string;
  threadId: string;
  title?: string;
  disabled?: boolean;
  onSend?: (prompt: string, assetIds: string[]) => void;
};

/** Ref hydration stays outside model context. Editing affects only the next attached selection. */
export function MusicAgentResult({ refId, threadId, title, disabled, onSend }: Props) {
  const { t } = useI18n();
  const player = useMusicPlayer();
  const result = $api.useQuery("get", "/api/v1/agent/refs/{id}/music", {
    params: { path: { id: refId }, query: { thread_id: threadId } },
  });
  const [order, setOrder] = useState<string[]>();
  const [excluded, setExcluded] = useState<Set<string>>(() => new Set());
  const [playlistTitle, setPlaylistTitle] = useState("");
  const [prompt, setPrompt] = useState("");
  const tracks = result.data?.tracks ?? [];
  const byId = new Map(tracks.map((track) => [trackId(track), track]));
  const ordered = (order ?? tracks.map(trackId)).flatMap((id) => {
    const track = byId.get(id);
    return track ? [track] : [];
  });
  const selected = ordered.filter((track) => !excluded.has(trackId(track)));
  const ids = selected.map(trackId);
  const move = (index: number, delta: number) => {
    const next = ordered.map(trackId);
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    setOrder(next);
  };
  if (result.isPending)
    return <p role="status">{t("music.agent.loading", "Loading music selection…")}</p>;
  if (result.isError)
    return (
      <p role="alert">
        {t("music.agent.unavailable", "This music selection is no longer available.")}
      </p>
    );
  return (
    <section
      className="my-3 rounded-xl border border-base-300 bg-base-100 p-4"
      aria-label={t("music.agent.selection", "Music selection")}
    >
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="font-semibold">{title || t("music.agent.selection", "Music selection")}</h3>
        <button
          type="button"
          className="btn btn-sm"
          disabled={!ids.length}
          onClick={() => player.playTracks(selected)}
        >
          <Play size={14} />
          {t("music.agent.audition", "Audition selection")}
        </button>
      </div>
      {result.data?.truncated && (
        <p className="mb-2 text-xs text-base-content/60">
          {t(
            "music.agent.bounded",
            "This is a bounded selection; refine it to explore more tracks.",
          )}
        </p>
      )}
      <ol className="max-h-80 space-y-1 overflow-y-auto">
        {ordered.map((track, index) => (
          <li
            key={trackId(track)}
            className="flex items-center gap-2 rounded-lg p-2 hover:bg-base-200"
          >
            {onSend && (
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={!excluded.has(trackId(track))}
                disabled={disabled}
                aria-label={t("music.agent.selectTrack", "Select {{title}}", {
                  title: trackTitle(track),
                })}
                onChange={() =>
                  setExcluded((previous) => {
                    const next = new Set(previous);
                    const id = trackId(track);
                    if (next.has(id)) next.delete(id);
                    else next.add(id);
                    return next;
                  })
                }
              />
            )}
            <button
              type="button"
              className="min-w-0 flex-1 text-left"
              onClick={() => player.playTracks(ordered, index)}
            >
              <span className="block truncate text-sm">{trackTitle(track)}</span>
              <span className="block truncate text-xs text-base-content/55">
                {trackArtist(track)}
              </span>
            </button>
            {onSend && (
              <>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs"
                  disabled={disabled || index === 0}
                  aria-label={t("music.agent.moveUp", "Move {{title}} up", {
                    title: trackTitle(track),
                  })}
                  onClick={() => move(index, -1)}
                >
                  <ArrowUp size={14} />
                </button>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs"
                  disabled={disabled || index === ordered.length - 1}
                  aria-label={t("music.agent.moveDown", "Move {{title}} down", {
                    title: trackTitle(track),
                  })}
                  onClick={() => move(index, 1)}
                >
                  <ArrowDown size={14} />
                </button>
              </>
            )}
          </li>
        ))}
      </ol>
      {onSend && (
        <div className="mt-3 space-y-2 border-t border-base-200 pt-3">
          <p className="text-xs text-base-content/60">
            {t("music.agent.selected", "{{count}} tracks selected, in the order shown", {
              count: ids.length,
            })}
          </p>
          <form
            className="flex gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (ids.length && prompt.trim() && !disabled) onSend(prompt.trim(), ids);
            }}
          >
            <input
              className="input input-sm min-w-0 flex-1"
              aria-label={t("music.agent.refine", "Refine selection")}
              placeholder={t("music.agent.refine", "Refine selection")}
              value={prompt}
              disabled={disabled}
              onChange={(event) => setPrompt(event.target.value)}
            />
            <button className="btn btn-sm" disabled={disabled || !ids.length || !prompt.trim()}>
              {t("music.agent.refine", "Refine selection")}
            </button>
          </form>
          <form
            className="flex gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (ids.length && playlistTitle.trim() && !disabled)
                onSend(
                  t(
                    "music.agent.savePrompt",
                    "Create a music playlist named “{{title}}” from exactly the attached selection, preserving its order. Ask me to confirm before saving.",
                    { title: playlistTitle.trim() },
                  ),
                  ids,
                );
            }}
          >
            <input
              className="input input-sm min-w-0 flex-1"
              aria-label={t("music.agent.playlistTitle", "Playlist title")}
              placeholder={t("music.agent.playlistTitle", "Playlist title")}
              maxLength={100}
              value={playlistTitle}
              disabled={disabled}
              onChange={(event) => setPlaylistTitle(event.target.value)}
            />
            <button
              className="btn btn-primary btn-sm"
              disabled={disabled || !ids.length || !playlistTitle.trim()}
            >
              {t("music.agent.save", "Save playlist…")}
            </button>
          </form>
        </div>
      )}
    </section>
  );
}

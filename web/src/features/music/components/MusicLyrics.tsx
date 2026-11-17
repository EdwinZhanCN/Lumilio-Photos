import { useEffect, useMemo, useRef, useState } from "react";
import { parseTimedLyrics, activeLyricIndex } from "../model/lyrics";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useMusicFeedback } from "../state/useMusicFeedback";

export default function MusicLyrics({
  trackId,
  editable = false,
  currentTime = 0,
  onSeek,
}: {
  trackId?: string;
  editable?: boolean;
  currentTime?: number;
  onSeek?: (time: number) => void;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const lyrics = $api.useQuery(
    "get",
    "/api/v1/music/tracks/{id}/lyrics",
    { params: { path: { id: trackId ?? "" } } },
    { enabled: Boolean(trackId) },
  );
  const save = $api.useMutation("put", "/api/v1/music/tracks/{id}/lyrics");
  const [draft, setDraft] = useState<{ id?: string; content: string; revision: number }>();
  const content = draft && draft.id === trackId ? draft.content : (lyrics.data?.content ?? "");
  const timed = useMemo(() => parseTimedLyrics(lyrics.data?.content ?? ""), [lyrics.data?.content]);
  const active = activeLyricIndex(timed, currentTime);
  const activeLine = useRef<HTMLButtonElement>(null);
  const { run, feedback, pending } = useMusicFeedback();
  useEffect(() => {
    activeLine.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [active]);
  return (
    <section className="space-y-3 p-4">
      <h2 className="font-semibold">{t("music.lyrics.title", "Lyrics")}</h2>
      {lyrics.isPending ? (
        <span className="loading loading-spinner" />
      ) : lyrics.isError ? (
        <button className="btn btn-error btn-sm" onClick={() => void lyrics.refetch()}>
          {t("common.retry", "Retry")}
        </button>
      ) : editable ? (
        <form
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            void run(async () => {
              await save.mutateAsync({
                params: { path: { id: trackId ?? "" } },
                body: {
                  content,
                  revision:
                    draft && draft.id === trackId ? draft.revision : (lyrics.data?.revision ?? 0),
                },
              });
              setDraft(undefined);
              await queryClient.invalidateQueries({
                queryKey: ["get", "/api/v1/music/tracks/{id}/lyrics"],
              });
            });
          }}
        >
          <p className="text-sm text-base-content/60">
            {t(
              "music.lyrics.hint",
              "Paste plain text or LRC lyrics. Timed lyrics follow playback.",
            )}
          </p>
          <textarea
            className="textarea textarea-bordered min-h-40 w-full"
            aria-label={t("music.lyrics.title", "Lyrics")}
            value={content}
            onChange={(event) =>
              setDraft((previous) => ({
                id: trackId,
                content: event.target.value,
                revision:
                  previous && previous.id === trackId
                    ? previous.revision
                    : (lyrics.data?.revision ?? 0),
              }))
            }
            maxLength={65536}
            disabled={pending}
          />
          <button className="btn btn-primary btn-sm" disabled={pending}>
            {t("music.actions.save", "Save changes")}
          </button>
        </form>
      ) : timed.length > 0 ? (
        <div className="space-y-3">
          {timed.map((line, index) => (
            <button
              type="button"
              key={`${line.time}-${index}`}
              ref={index === active ? activeLine : undefined}
              aria-current={index === active ? "true" : undefined}
              disabled={!onSeek}
              onClick={() => onSeek?.(line.time)}
              className={`block w-full text-left text-xl leading-relaxed ${index === active ? "font-bold text-primary" : "opacity-50"}`}
            >
              {line.text || "♪"}
            </button>
          ))}
        </div>
      ) : (
        <p className="whitespace-pre-wrap text-sm leading-7">
          {lyrics.data?.content ||
            t("music.lyrics.empty", "No local lyrics. Add them in track details.")}
        </p>
      )}
      {feedback}
    </section>
  );
}

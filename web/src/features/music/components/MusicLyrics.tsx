import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useMusicFeedback } from "../state/useMusicFeedback";

export default function MusicLyrics({
  trackId,
  editable = false,
}: {
  trackId?: string;
  editable?: boolean;
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
  const [content, setContent] = useState("");
  const { run, feedback, pending } = useMusicFeedback();
  useEffect(() => {
    setContent(lyrics.data?.content ?? "");
  }, [lyrics.data]);
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
                body: { content, revision: lyrics.data?.revision ?? 0 },
              });
              await queryClient.invalidateQueries({
                queryKey: ["get", "/api/v1/music/tracks/{id}/lyrics"],
              });
            });
          }}
        >
          <p className="text-sm text-base-content/60">
            {t(
              "music.lyrics.hint",
              "Paste lyrics to keep them locally. Original audio files are unchanged.",
            )}
          </p>
          <textarea
            className="textarea textarea-bordered min-h-40 w-full"
            aria-label={t("music.lyrics.title", "Lyrics")}
            value={content}
            onChange={(event) => setContent(event.target.value)}
            maxLength={65536}
          />
          <button className="btn btn-primary btn-sm" disabled={pending}>
            {t("music.actions.save", "Save changes")}
          </button>
        </form>
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

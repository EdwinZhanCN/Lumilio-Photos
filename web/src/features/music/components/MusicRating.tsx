import { useQueryClient } from "@tanstack/react-query";
import { Star } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useMusicMutations, useMusicTrack } from "../api/useMusic";
import type { MusicTrack } from "../model/music";
import { useMusicFeedback } from "../state/useMusicFeedback";

export default function MusicRating({ track }: { track: MusicTrack }) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const query = useMusicTrack(track.track_id, track);
  const mutation = $api.useMutation("put", "/api/v1/assets/{id}/rating");
  const { invalidateMusic } = useMusicMutations();
  const { run, feedback, pending } = useMusicFeedback();
  const rating = query.data?.rating ?? 0;
  const save = (value: number) =>
    void run(async () => {
      if (!track.track_id) return;
      await mutation.mutateAsync({
        params: { path: { id: track.track_id } },
        body: { rating: value },
      });
      await Promise.all([
        invalidateMusic(),
        ...["/api/v1/assets/list", "/api/v1/assets/search", "/api/v1/assets/{id}"].map((path) =>
          queryClient.invalidateQueries({ queryKey: ["get", path] }),
        ),
      ]);
    });
  return (
    <div className="space-y-2">
      <div
        role="group"
        aria-label={t("music.rating.title", "Rating")}
        className="flex flex-wrap items-center gap-1"
      >
        {[1, 2, 3, 4, 5].map((value) => (
          <button
            key={value}
            type="button"
            className="btn btn-ghost btn-xs btn-circle"
            disabled={pending || !track.track_id}
            aria-label={t("music.rating.stars", "Rate {{count}} stars", { count: value })}
            aria-pressed={rating === value}
            onClick={() => save(value)}
          >
            <Star
              className="size-4 text-warning"
              fill={value <= rating ? "currentColor" : "none"}
            />
          </button>
        ))}
        <button
          type="button"
          className="btn btn-ghost btn-xs"
          disabled={pending || rating === 0}
          onClick={() => save(0)}
        >
          {t("music.rating.clear", "Clear rating")}
        </button>
      </div>
      {feedback}
    </div>
  );
}

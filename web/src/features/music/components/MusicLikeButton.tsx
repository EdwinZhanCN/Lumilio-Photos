import { Heart } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useMusicMutations, useMusicTrack } from "../api/useMusic";
import type { MusicTrack } from "../model/music";

export default function MusicLikeButton({
  trackId,
  track: initialTrack,
  showLabel = false,
}: {
  trackId?: string;
  track?: MusicTrack;
  showLabel?: boolean;
}) {
  const { t } = useI18n();
  const track = useMusicTrack(trackId, initialTrack);
  const { invalidateMusic } = useMusicMutations();
  const mutation = $api.useMutation("put", "/api/v1/assets/{id}/like");
  const liked = track.data?.liked ?? false;
  const toggle = async () => {
    if (!trackId || !track.data || mutation.isPending) return;
    try {
      await mutation.mutateAsync({ params: { path: { id: trackId } }, body: { liked: !liked } });
      await invalidateMusic();
    } catch {
      // The mutation error remains visible and the last confirmed state is retained.
    }
  };
  return (
    <span className="inline-flex shrink-0 items-center gap-1">
      <button
        type="button"
        className={`btn btn-ghost btn-sm ${showLabel ? "w-full justify-start" : "btn-circle"} ${liked ? "text-primary" : "text-base-content/50"}`}
        aria-label={liked ? t("music.actions.unlike", "Unlike") : t("music.actions.like", "Like")}
        aria-pressed={liked}
        disabled={!track.data || mutation.isPending}
        onClick={() => void toggle()}
      >
        <Heart className="size-4" fill={liked ? "currentColor" : "none"} />
        {showLabel &&
          (liked ? t("music.actions.unlike", "Unlike") : t("music.actions.like", "Like"))}
      </button>
      {mutation.isError && (
        <span role="alert" className="text-xs text-error">
          {t("music.likeError", "Could not save. Try again.")}
        </span>
      )}
    </span>
  );
}

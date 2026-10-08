import { Heart } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useMusicMutations } from "../api/useMusic";
import { useMusicFeedback } from "../state/useMusicFeedback";

export default function MusicFavoriteButton({
  kind,
  id,
  favorite,
  revision,
}: {
  kind: "album" | "artist";
  id?: string;
  favorite?: boolean;
  revision?: number;
}) {
  const { t } = useI18n();
  const { updateAlbum, updateArtist, invalidateMusic } = useMusicMutations();
  const { run, feedback, pending } = useMusicFeedback();
  return (
    <div className="flex items-center gap-2">
      <button
        className="btn btn-ghost btn-circle btn-sm"
        type="button"
        aria-pressed={Boolean(favorite)}
        disabled={!id || pending}
        aria-label={
          favorite
            ? t("music.actions.unfavorite", "Remove from favorites")
            : t("music.actions.favorite", "Add to favorites")
        }
        onClick={() =>
          void run(async () => {
            const params = {
              params: { path: { id: id ?? "" } },
              body: { favorite: !favorite, revision },
            };
            if (kind === "album") await updateAlbum.mutateAsync(params);
            else await updateArtist.mutateAsync(params);
            await invalidateMusic();
          })
        }
      >
        <Heart className={`size-4 ${favorite ? "fill-current text-primary" : ""}`} />
      </button>
      {feedback}
    </div>
  );
}

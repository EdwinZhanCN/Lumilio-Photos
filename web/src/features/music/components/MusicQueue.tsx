import { Play, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { trackArtist, trackTitle } from "../model/music";
import { useMusicPlayer } from "../state/MusicPlayerProvider";

export default function MusicQueue() {
  const { t } = useI18n();
  const { queue, currentIndex, playQueueIndex, removeQueueIndex } = useMusicPlayer();
  return (
    <div className="w-full">
      <h2 className="mb-6 text-3xl font-bold">{t("music.queue.title", "Play queue")}</h2>
      <ol>
        {queue.map((item, index) => (
          <li
            key={`${item.entryId ?? item.track.track_id}-${index}`}
            className={`flex items-center gap-3 rounded-lg px-2 py-1 ${index === currentIndex ? "bg-primary/10 text-primary" : ""}`}
          >
            <button
              type="button"
              className="btn btn-ghost btn-circle btn-sm"
              disabled={!item.available}
              onClick={() => playQueueIndex(index)}
              aria-label={t("music.player.play", "Play")}
            >
              <Play className="size-4" />
            </button>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm">{trackTitle(item.track)}</p>
              <p className="truncate text-xs opacity-60">
                {item.available
                  ? trackArtist(item.track)
                  : t("music.queue.unavailable", "Unavailable")}
              </p>
            </div>
            <button
              type="button"
              className="btn btn-ghost btn-circle btn-sm"
              onClick={() => removeQueueIndex(index)}
              aria-label={t("music.queue.remove", "Remove from queue")}
            >
              <X className="size-4" />
            </button>
          </li>
        ))}
      </ol>
    </div>
  );
}

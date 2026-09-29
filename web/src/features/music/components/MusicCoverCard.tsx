import type { ReactNode } from "react";
import { Play } from "lucide-react";
import Link from "./MusicLink";
import { useI18n } from "@/lib/i18n";
import { useMusicPlayer } from "../state/MusicPlayerProvider";
import type { MusicPlaybackSource } from "../model/music";
export default function MusicCoverCard({
  to,
  title,
  subtitle,
  source,
  children,
  round = false,
}: {
  to: string;
  title: string;
  subtitle?: ReactNode;
  source: MusicPlaybackSource;
  children: ReactNode;
  round?: boolean;
}) {
  const { t } = useI18n();
  const { playSource } = useMusicPlayer();
  return (
    <article className={`music-cover-card ${round ? "text-center" : ""}`}>
      <div className="music-cover-art">
        <Link to={to} aria-label={title}>
          {children}
        </Link>
        <button
          className="music-cover-play"
          aria-label={`${t("music.player.play", "Play")} ${title}`}
          onClick={() => void playSource(source)}
        >
          <Play fill="currentColor" />
        </button>
      </div>
      <Link
        to={to}
        className="mt-3 block line-clamp-2 text-base leading-5 font-semibold hover:underline"
      >
        {title}
      </Link>
      {subtitle && <div className="mt-1 truncate text-xs opacity-60">{subtitle}</div>}
    </article>
  );
}

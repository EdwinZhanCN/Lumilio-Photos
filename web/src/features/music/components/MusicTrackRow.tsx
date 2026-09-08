import { useId, useRef } from "react";
import { ListPlus, MoreHorizontal, Play } from "lucide-react";
import Link from "./MusicLink";
import MusicPlaylistAction from "./MusicPlaylistAction";
import MusicLikeButton from "./MusicLikeButton";
import MusicArtwork from "./MusicArtwork";
import { useI18n } from "@/lib/i18n";
import {
  formatMusicDuration,
  trackAlbum,
  trackArtist,
  trackId,
  trackTitle,
  type MusicPlaybackSource,
  type MusicTrack,
} from "../model/music";
import { useMusicPlayer } from "../state/MusicPlayerProvider";
import "./MusicLibrary.css";

type MusicTrackRowProps = {
  track: MusicTrack;
  compact?: boolean;
  onRemove?: () => void;
  entryId?: string;
  index?: number;
  source?: MusicPlaybackSource;
};
export default function MusicTrackRow({
  track,
  entryId,
  index,
  source,
  compact = false,
  onRemove,
}: MusicTrackRowProps) {
  const { t } = useI18n();
  const { current, playTrack, enqueue } = useMusicPlayer();
  const id = trackId(track);
  const menuId = useId();
  const menu = useRef<HTMLDivElement>(null);
  const isCurrent =
    current?.track && trackId(current.track) === id && (!entryId || current.entryId === entryId);
  const play = () => {
    if (id) void playTrack(track, source, entryId);
  };
  const albumRow = source?.kind === "album";
  return (
    <div
      className={`music-track group ${compact ? "compact" : ""} ${isCurrent ? "is-current" : ""}`}
      onDoubleClick={(event) => {
        if (!(event.target as HTMLElement).closest("button,a,input")) play();
      }}
      onContextMenu={(event) => {
        event.preventDefault();
        if (menu.current) {
          menu.current.style.left = `${Math.max(8, Math.min(event.clientX, window.innerWidth - 248))}px`;
          menu.current.style.top = `${Math.max(8, Math.min(event.clientY, window.innerHeight - 310))}px`;
          menu.current.showPopover();
        }
      }}
    >
      {albumRow ? (
        <button
          className="w-8 shrink-0 text-sm opacity-50"
          onClick={play}
          aria-label={t("music.player.play", "Play")}
        >
          {index == null ? <Play className="size-4" /> : index + 1}
        </button>
      ) : (
        <Link
          className="music-row-art shrink-0"
          to={track.album_id ? `/music/albums/${track.album_id}` : `/music/tracks/${id}`}
        >
          <MusicArtwork assetId={id} alt={trackTitle(track)} size="sm" />
        </Link>
      )}
      <div className="min-w-0 flex-1">
        <span className="music-track-title">{trackTitle(track)}</span>
        <p className="music-track-artist">
          {track.artists?.length
            ? track.artists.map((artist, position) => (
                <span key={artist.artist_id ?? position}>
                  {position > 0 && " · "}
                  <Link className="hover:underline" to={`/music/artists/${artist.artist_id}`}>
                    {artist.display_name}
                  </Link>
                </span>
              ))
            : trackArtist(track)}
        </p>
      </div>
      {!compact && !albumRow && (
        <Link
          className="hidden w-1/4 truncate text-sm opacity-60 hover:underline sm:block"
          to={track.album_id ? `/music/albums/${track.album_id}` : "/music?view=albums"}
        >
          {trackAlbum(track)}
        </Link>
      )}
      {!compact && <MusicLikeButton trackId={id} track={track} />}
      {!compact && (
        <span className="w-12 shrink-0 text-right text-sm opacity-50">
          {formatMusicDuration(track.duration)}
        </span>
      )}
      <button
        popoverTarget={menuId}
        className="btn btn-ghost btn-circle btn-xs opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"
        aria-label={t("music.actions.more", "More options")}
        onClick={(event) => {
          const rect = event.currentTarget.getBoundingClientRect();
          if (menu.current) {
            menu.current.style.left = `${Math.max(8, Math.min(rect.left, window.innerWidth - 248))}px`;
            menu.current.style.top = `${Math.max(8, Math.min(rect.bottom, window.innerHeight - 310))}px`;
          }
        }}
      >
        <MoreHorizontal className="size-4" />
      </button>
      <div
        id={menuId}
        ref={menu}
        popover="auto"
        className="music-track-menu"
        onDoubleClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center gap-3 border-b border-base-300 px-3 py-3">
          <MusicArtwork assetId={id} alt={trackTitle(track)} size="sm" />
          <div className="min-w-0">
            <p className="truncate font-semibold">{trackTitle(track)}</p>
            <p className="truncate text-xs opacity-50">{trackArtist(track)}</p>
          </div>
        </div>
        <button
          onClick={() => {
            play();
            menu.current?.hidePopover();
          }}
        >
          <Play className="size-4" />
          {t("music.player.play", "Play")}
        </button>
        <button
          onClick={() => {
            enqueue(track);
            menu.current?.hidePopover();
          }}
        >
          <ListPlus className="size-4" />
          {t("music.queue.add", "Add to queue")}
        </button>
        <div className="grid gap-1 px-1 py-1">
          <MusicLikeButton trackId={id} track={track} showLabel />
          <MusicPlaylistAction trackId={id} showLabel />
        </div>
        {onRemove && (
          <button
            className="text-error"
            onClick={() => {
              menu.current?.hidePopover();
              onRemove();
            }}
          >
            {t("music.playlist.remove", "Remove entry")}
          </button>
        )}
        <Link className="block px-3 py-2 font-semibold" to={`/music/tracks/${id}`}>
          {t("music.track.details", "Track details")}
        </Link>
      </div>
    </div>
  );
}

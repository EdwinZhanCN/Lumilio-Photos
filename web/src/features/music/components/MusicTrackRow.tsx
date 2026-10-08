import { useRef } from "react";
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
  /** Show a track's play position/index instead of its artwork (default false = Tracks-list style with artwork). */
  showTrackNumber?: boolean;
  /** Show the album-name column (default true; hide when the album column is redundant, e.g. album detail). */
  showAlbumColumn?: boolean;
};
export default function MusicTrackRow({
  track,
  entryId,
  index,
  source,
  compact = false,
  onRemove,
  showTrackNumber = false,
  showAlbumColumn = true,
}: MusicTrackRowProps) {
  const { t } = useI18n();
  const { current, playTrack, enqueue } = useMusicPlayer();
  const id = trackId(track);
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const isCurrent =
    current?.track && trackId(current.track) === id && (!entryId || current.entryId === entryId);
  const play = () => {
    if (id) void playTrack(track, source, entryId);
  };
  const albumColumn = !compact && showAlbumColumn;
  return (
    <div
      className={`music-track group ${compact ? "compact" : ""} ${isCurrent ? "is-current" : ""}`}
      onDoubleClick={(event) => {
        if (!(event.target as HTMLElement).closest("button,a,input")) play();
      }}
    >
      {showTrackNumber ? (
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
      {albumColumn && (
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
      <details ref={detailsRef} className="dropdown dropdown-end group">
        <summary
          className="btn btn-ghost btn-circle btn-xs opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 group-open:opacity-100"
          aria-label={t("music.actions.more", "More options")}
        >
          <MoreHorizontal className="size-4" />
        </summary>
        <div
          className="dropdown-content music-row-menu bg-base-200 rounded-box z-dropdown w-56 p-1.5 shadow-xl"
          onDoubleClick={(event) => event.stopPropagation()}
        >
          <div className="mb-1 flex items-center gap-3 border-b border-base-300 px-3 py-3">
            <MusicArtwork assetId={id} alt={trackTitle(track)} size="sm" />
            <div className="min-w-0">
              <p className="truncate font-semibold">{trackTitle(track)}</p>
              <p className="truncate text-xs opacity-50">{trackArtist(track)}</p>
            </div>
          </div>
          <button
            className="btn btn-ghost btn-sm justify-start w-full"
            onClick={() => {
              detailsRef.current?.removeAttribute("open");
              play();
            }}
          >
            <Play className="size-4" />
            {t("music.player.play", "Play")}
          </button>
          <button
            className="btn btn-ghost btn-sm justify-start w-full"
            onClick={() => {
              detailsRef.current?.removeAttribute("open");
              enqueue(track);
            }}
          >
            <ListPlus className="size-4" />
            {t("music.queue.add", "Add to queue")}
          </button>
          <MusicLikeButton trackId={id} track={track} showLabel />
          <MusicPlaylistAction trackId={id} showLabel />
          {onRemove && (
            <button
              className="btn btn-ghost btn-sm justify-start w-full text-error"
              onClick={() => {
                detailsRef.current?.removeAttribute("open");
                onRemove();
              }}
            >
              {t("music.playlist.remove", "Remove entry")}
            </button>
          )}
          <Link
            className="btn btn-ghost btn-sm justify-start w-full"
            to={`/music/tracks/${id}`}
            onClick={() => detailsRef.current?.removeAttribute("open")}
          >
            {t("music.track.details", "Track details")}
          </Link>
        </div>
      </details>
    </div>
  );
}

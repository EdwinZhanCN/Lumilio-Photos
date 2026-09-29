import { useEffect, useRef, useState, type CSSProperties } from "react";
import {
  ChevronDown,
  ChevronUp,
  ListMusic,
  Pause,
  Play,
  Repeat,
  Repeat1,
  Shuffle,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import { useMusicTrack } from "../api/useMusic";
import MusicWaveform from "./MusicWaveform";
import MusicRating from "./MusicRating";
import MusicLyrics from "./MusicLyrics";
import MusicQueue from "./MusicQueue";
import MusicLikeButton from "./MusicLikeButton";
import MusicArtwork from "./MusicArtwork";
import { useI18n } from "@/lib/i18n";
import { trackArtist, trackTitle } from "../model/music";
import { useMusicPlayer } from "../state/MusicPlayerProvider";
import "./MusicLibrary.css";

export default function MusicPlayerDock() {
  const { t } = useI18n();
  const [lyricsOpen, setLyricsOpen] = useState(false);
  const [queueOpen, setQueueOpen] = useState(false);
  const previousVolume = useRef(0.8);
  const lyricsDialog = useRef<HTMLDialogElement>(null);
  const {
    current,
    isPlaying,
    currentTime,
    duration,
    volume,
    repeat,
    shuffle,
    isLoading,
    error,
    toggle,
    next,
    previous,
    seek,
    setVolume,
    toggleShuffle,
    cycleRepeat,
    clear,
    queue,
  } = useMusicPlayer();
  const currentQuery = useMusicTrack(current?.track.track_id);
  const displayTrack = currentQuery.data ?? current?.track;
  useEffect(() => {
    if (lyricsOpen) lyricsDialog.current?.showModal();
    else lyricsDialog.current?.close();
  }, [lyricsOpen]);
  if (!current && queue.length === 0 && !isLoading && !error) return null;
  const max = duration > 0 ? duration : (current?.track.duration ?? 0);
  const progress = max > 0 ? Math.min(max, currentTime) : 0;
  const transport = (
    <div className="music-transport">
      <button aria-label={t("music.player.previous", "Previous track")} onClick={previous}>
        <SkipBack fill="currentColor" />
      </button>
      <button
        className="music-transport-play"
        disabled={!current?.available || isLoading}
        aria-label={isPlaying ? t("music.player.pause", "Pause") : t("music.player.play", "Play")}
        onClick={toggle}
      >
        {isPlaying ? <Pause fill="currentColor" /> : <Play fill="currentColor" />}
      </button>
      <button aria-label={t("music.player.next", "Next track")} onClick={next}>
        <SkipForward fill="currentColor" />
      </button>
    </div>
  );
  return (
    <>
      {queueOpen && (
        <section className="music-queue-page fixed inset-x-0 top-16 bottom-16 z-overlay overflow-auto bg-base-100">
          <div className="flex justify-end">
            <button
              className="btn btn-ghost btn-circle"
              aria-label={t("common.close", "Close")}
              onClick={() => setQueueOpen(false)}
            >
              <X />
            </button>
          </div>
          <MusicQueue />
          <button
            className="btn btn-ghost mt-6"
            onClick={() => {
              clear();
              setQueueOpen(false);
            }}
          >
            {t("music.player.close", "Close player")}
          </button>
        </section>
      )}
      <section
        aria-label={t("music.player.label", "Music player")}
        className="music-player fixed inset-x-0 bottom-0 z-overlay bg-base-100/95 backdrop-blur-md"
      >
        <input
          type="range"
          min={0}
          max={max || 1}
          step={0.1}
          value={progress}
          onChange={(event) => seek(Number(event.target.value))}
          className="music-seek"
          style={
            { "--music-progress": `${max > 0 ? (progress / max) * 100 : 0}%` } as CSSProperties
          }
          aria-label={t("music.player.seek", "Seek")}
          disabled={!current?.available || max <= 0}
        />
        <div className="music-player-body">
          <div className="music-player-info">
            <MusicArtwork
              assetId={current?.track.track_id}
              alt={displayTrack ? trackTitle(displayTrack) : ""}
              size="sm"
            />
            <div className="min-w-0">
              <p className="truncate text-base font-semibold">
                {isLoading
                  ? t("music.player.loading", "Preparing queue…")
                  : displayTrack
                    ? trackTitle(displayTrack)
                    : t("music.player.empty", "Nothing is playing")}
              </p>
              <p className="truncate text-xs opacity-60">
                {error || (displayTrack ? trackArtist(displayTrack) : "")}
              </p>
            </div>
            <MusicLikeButton trackId={current?.track.track_id} />
          </div>
          {transport}
          <div className="music-player-tools">
            <button
              aria-label={t("music.queue.title", "Play queue")}
              aria-expanded={queueOpen}
              onClick={() => setQueueOpen(!queueOpen)}
            >
              <ListMusic />
            </button>
            <button
              aria-label={t("music.player.repeat", "Repeat")}
              aria-pressed={repeat !== "off"}
              onClick={cycleRepeat}
            >
              {repeat === "one" ? <Repeat1 /> : <Repeat />}
            </button>
            <button
              aria-label={t("music.player.shuffle", "Shuffle")}
              aria-pressed={shuffle}
              onClick={toggleShuffle}
            >
              <Shuffle />
            </button>
            <button
              className="music-volume"
              aria-label={
                volume === 0 ? t("music.player.unmute", "Unmute") : t("music.player.mute", "Mute")
              }
              onClick={() => {
                if (volume > 0) {
                  previousVolume.current = volume;
                  setVolume(0);
                } else setVolume(previousVolume.current);
              }}
            >
              {volume === 0 ? <VolumeX /> : <Volume2 />}
            </button>
            <input
              className="music-volume"
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={volume}
              onChange={(event) => setVolume(Number(event.target.value))}
              aria-label={t("music.player.volume", "Volume")}
            />
            <button
              disabled={!current}
              aria-label={t("music.lyrics.title", "Lyrics")}
              aria-expanded={lyricsOpen}
              onClick={() => setLyricsOpen(true)}
            >
              <ChevronUp />
            </button>
          </div>
        </div>
      </section>
      <dialog
        ref={lyricsDialog}
        className="music-lyrics-dialog"
        onClose={() => setLyricsOpen(false)}
      >
        <button
          className="music-lyrics-close btn btn-ghost btn-circle"
          aria-label={t("common.close", "Close")}
          onClick={() => setLyricsOpen(false)}
        >
          <ChevronDown />
        </button>
        <div className="music-lyrics-layout">
          <div className="music-lyrics-cover">
            <MusicArtwork
              assetId={current?.track.track_id}
              alt={displayTrack ? trackTitle(displayTrack) : ""}
              size="cover"
            />
            <h2 className="mt-8 text-2xl font-bold">{displayTrack && trackTitle(displayTrack)}</h2>
            <p className="mt-2 opacity-50">{displayTrack && trackArtist(displayTrack)}</p>
            <div className="mt-8">{transport}</div>
            <MusicWaveform
              trackId={current?.track.track_id}
              currentTime={progress}
              duration={max}
              onSeek={seek}
            />
            {displayTrack && <MusicRating track={displayTrack} />}
          </div>
          <div className="music-lyrics-content">
            <MusicLyrics
              key={current?.track.track_id}
              trackId={current?.track.track_id}
              currentTime={currentTime}
              onSeek={seek}
            />
          </div>
        </div>
      </dialog>
    </>
  );
}

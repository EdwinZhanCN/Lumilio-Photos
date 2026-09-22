import { useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { formatMusicDuration } from "../model/music";

export default function MusicWaveform({
  trackId,
  currentTime,
  duration,
  onSeek,
}: {
  trackId?: string;
  currentTime: number;
  duration: number;
  onSeek: (time: number) => void;
}) {
  const { t } = useI18n();
  const [failed, setFailed] = useState<string>();
  const value = Math.max(0, Math.min(currentTime, duration || 0));
  return (
    <section className="mt-6 space-y-2" aria-label={t("music.waveform.title", "Waveform")}>
      <div className="relative h-16 overflow-hidden rounded-lg bg-base-200">
        {trackId && failed !== trackId && (
          <img
            alt=""
            className="h-full w-full object-fill opacity-70"
            src={assetUrls.getThumbnailUrl(trackId, "waveform")}
            onError={() => setFailed(trackId)}
          />
        )}
        <div
          className="pointer-events-none absolute inset-y-0 left-0 border-r-2 border-primary bg-primary/15"
          style={{ width: `${duration > 0 ? (value / duration) * 100 : 0}%` }}
        />
        <input
          type="range"
          min={0}
          max={duration || 1}
          step={0.1}
          value={value}
          disabled={!trackId || duration <= 0}
          onChange={(event) => onSeek(Number(event.target.value))}
          aria-label={t("music.player.seek", "Seek")}
          aria-valuetext={formatMusicDuration(value)}
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0 focus:opacity-100"
        />
      </div>
      <div className="flex justify-between text-xs opacity-60">
        <span>{formatMusicDuration(value)}</span>
        <span>{formatMusicDuration(duration)}</span>
      </div>
      {failed === trackId && trackId && (
        <p className="text-xs opacity-60">
          {t("music.waveform.unavailable", "Waveform unavailable. You can still seek.")}
        </p>
      )}
    </section>
  );
}

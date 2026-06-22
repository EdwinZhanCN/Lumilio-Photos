import { createPortal } from "react-dom";
import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { ChevronLeft, ChevronRight, Clock, Play } from "lucide-react";
import type { Asset } from "@/lib/assets/types";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { useI18n } from "@/lib/i18n.tsx";
import { isMockWidgetSource } from "./mockWidgetData";
import type { WidgetProps } from "./types";
import { useWidgetAssetsPreview } from "./useWidgetAssets";
import { WidgetAssetThumbnail } from "./WidgetAssetThumbnail";

const INLINE_STORY_COUNT = 10;
const BOARD_STORY_COUNT = 18;
const AUTOPLAY_MS = 3800;

/** The storyline widget plays a ref as an Instagram-style story: a full-bleed
 * sequence with a segmented progress bar, tap-through navigation and an
 * auto-advancing board player. Inline renders a tappable story cover. */
export function StorylineWidget({ source, variant, count, title }: WidgetProps) {
  const { t } = useI18n();
  const limit = variant === "board" ? BOARD_STORY_COUNT : INLINE_STORY_COUNT;
  const { assets, isLoading, isError } = useWidgetAssetsPreview(source, limit);
  const [carouselAssetId, setCarouselAssetId] = useState<string>();
  const canOpenCarousel = !isMockWidgetSource(source);

  const slideIndex = useMemo(
    () => (carouselAssetId ? assets.findIndex((asset) => asset.asset_id === carouselAssetId) : -1),
    [assets, carouselAssetId],
  );

  if (isLoading) return <StoryShell variant={variant} title={title} loading />;
  if (isError || assets.length === 0) {
    return (
      <StoryShell variant={variant} title={title}>
        <div className="flex h-24 items-center justify-center text-center text-xs text-base-content/50">
          {t("lumilio.widgets.storylineUnavailable", "This storyline is unavailable.")}
        </div>
      </StoryShell>
    );
  }

  const open = (asset: Asset) => {
    if (canOpenCarousel && asset.asset_id) setCarouselAssetId(asset.asset_id);
  };

  return (
    <StoryShell variant={variant} title={title}>
      {variant === "board" ? (
        <StoryPlayer assets={assets} source={source} count={count} onOpen={open} />
      ) : (
        <StoryCover assets={assets} source={source} count={count} onOpen={open} />
      )}

      {carouselAssetId &&
        assets.length > 0 &&
        createPortal(
          <FullScreenCarousel
            photos={assets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={() => setCarouselAssetId(undefined)}
            onNavigate={setCarouselAssetId}
          />,
          document.body,
        )}
    </StoryShell>
  );
}

function StoryShell({
  variant,
  title,
  loading = false,
  children,
}: {
  variant: WidgetProps["variant"];
  title?: string;
  loading?: boolean;
  children?: ReactNode;
}) {
  if (variant === "board") {
    return <div className="h-full">{loading ? <div className="skeleton h-full w-full" /> : children}</div>;
  }
  return (
    <div className="my-3 space-y-2">
      {title && <div className="text-sm font-medium text-base-content/80">{title}</div>}
      {loading ? (
        <div className="skeleton aspect-[4/5] w-44 rounded-2xl" />
      ) : (
        children
      )}
    </div>
  );
}

/** Board: an auto-advancing, tap-through story player that fills the cell. */
function StoryPlayer({
  assets,
  source,
  count,
  onOpen,
}: {
  assets: Asset[];
  source: WidgetProps["source"];
  count: number;
  onOpen: (asset: Asset) => void;
}) {
  const { t, i18n } = useI18n();
  const [index, setIndex] = useState(0);
  const [progress, setProgress] = useState(0);
  const [paused, setPaused] = useState(false);

  const safeIndex = Math.min(index, assets.length - 1);
  const current = assets[safeIndex];

  // Auto-advance with a smooth segment fill; restarts whenever the active
  // slide changes, freezes while paused.
  useEffect(() => {
    if (paused || assets.length <= 1) return;
    setProgress(0);
    const start = performance.now();
    let raf = 0;
    const tick = (now: number) => {
      const ratio = (now - start) / AUTOPLAY_MS;
      if (ratio >= 1) {
        setIndex((i) => (i + 1) % assets.length);
        return;
      }
      setProgress(ratio);
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [safeIndex, paused, assets.length]);

  const go = (next: number) => {
    setIndex((next + assets.length) % assets.length);
  };

  return (
    <div
      className="group relative h-full select-none overflow-hidden bg-black"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
    >
      <WidgetAssetThumbnail
        key={current?.asset_id}
        asset={current}
        source={source}
        className="absolute inset-0 h-full w-full animate-[storyfade_400ms_ease-out] object-cover"
      />
      <style>{"@keyframes storyfade{from{opacity:.35}to{opacity:1}}"}</style>
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-black/55 via-transparent to-black/70" />

      {/* Segmented progress bar */}
      <div className="absolute inset-x-0 top-0 flex gap-1 p-2.5">
        {assets.map((asset, i) => (
          <span key={asset.asset_id ?? i} className="h-[3px] flex-1 overflow-hidden rounded-full bg-white/30">
            <span
              className="block h-full rounded-full bg-white"
              style={{
                width: i < safeIndex ? "100%" : i === safeIndex ? `${Math.round(progress * 100)}%` : "0%",
              }}
            />
          </span>
        ))}
      </div>

      {/* Tap zones */}
      <button
        type="button"
        aria-label={t("lumilio.widgets.storyline.prev", "Previous")}
        className="absolute inset-y-0 left-0 flex w-1/3 items-center justify-start pl-2 text-white/0 transition-colors hover:text-white/80"
        onClick={() => go(safeIndex - 1)}
      >
        <ChevronLeft size={22} />
      </button>
      <button
        type="button"
        aria-label={t("lumilio.widgets.storyline.next", "Next")}
        className="absolute inset-y-0 right-0 flex w-1/3 items-center justify-end pr-2 text-white/0 transition-colors hover:text-white/80"
        onClick={() => go(safeIndex + 1)}
      >
        <ChevronRight size={22} />
      </button>

      {/* Caption */}
      <button
        type="button"
        className="absolute inset-x-0 bottom-0 cursor-pointer px-4 pb-4 pt-10 text-left"
        onClick={() => current && onOpen(current)}
      >
        <div className="flex items-center gap-2 text-xs font-medium text-white/85">
          <span className="rounded-full bg-white/20 px-2 py-0.5 backdrop-blur-sm">
            {safeIndex + 1} / {assets.length}
          </span>
          {count > assets.length && (
            <span className="text-white/60">
              {t("lumilio.widgets.storyline.assets", "{{count}} assets", { count })}
            </span>
          )}
        </div>
        <div className="mt-1.5 flex items-center gap-1.5 text-sm text-white">
          <Clock size={14} className="opacity-80" />
          {formatAssetMoment(current, i18n.language)}
        </div>
      </button>
    </div>
  );
}

/** Inline: a compact story cover that opens the full sequence on tap. */
function StoryCover({
  assets,
  source,
  count,
  onOpen,
}: {
  assets: Asset[];
  source: WidgetProps["source"];
  count: number;
  onOpen: (asset: Asset) => void;
}) {
  const { t } = useI18n();
  const cover = assets[0];
  const segments = Math.min(assets.length, 6);

  return (
    <button
      type="button"
      className="group relative block aspect-[4/5] w-44 select-none overflow-hidden rounded-2xl bg-base-200 text-left"
      onClick={() => cover && onOpen(cover)}
    >
      <WidgetAssetThumbnail
        asset={cover}
        source={source}
        className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
      />
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-black/45 via-transparent to-black/65" />

      <div className="absolute inset-x-0 top-0 flex gap-1 p-2">
        {Array.from({ length: segments }).map((_, i) => (
          <span
            key={i}
            className={`h-[3px] flex-1 rounded-full ${i === 0 ? "bg-white" : "bg-white/35"}`}
          />
        ))}
      </div>

      <div className="absolute inset-0 grid place-items-center">
        <span className="grid size-11 place-items-center rounded-full bg-white/25 text-white backdrop-blur-sm transition-transform duration-200 group-hover:scale-110">
          <Play size={18} className="translate-x-0.5 fill-white" />
        </span>
      </div>

      <div className="absolute inset-x-0 bottom-0 px-3 pb-3 pt-8 text-white">
        <div className="text-sm font-semibold">
          {t("lumilio.widgets.storyline.story", "Story")}
        </div>
        <div className="text-xs text-white/75">
          {t("lumilio.widgets.storyline.assets", "{{count}} assets", { count })}
        </div>
      </div>
    </button>
  );
}

function formatAssetMoment(asset: Asset | undefined, locale?: string): string {
  const raw = asset?.taken_time ?? asset?.upload_time;
  if (!raw) return asset?.original_filename ?? "";
  return new Intl.DateTimeFormat(locale, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(raw));
}

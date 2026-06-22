import { createPortal } from "react-dom";
import { Flame, Images, MousePointerClick } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent, ReactNode } from "react";
import type { Asset } from "@/lib/assets/types";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { useI18n } from "@/lib/i18n.tsx";
import type { AgentRefDTO } from "../types";
import { isMockWidgetSource } from "./mockWidgetData";
import {
  bucketStartDate,
  compressFacetBuckets,
  formatTimeBucketTitle,
  inferFacetGranularity,
  shortTimeBucket,
} from "./timeBucketLabels";
import type { FacetGranularity } from "./timeBucketLabels";
import type { WidgetProps } from "./types";
import { useWidgetAssetsPreview } from "./useWidgetAssets";
import { useWidgetMetadata } from "./useWidgetMetadata";
import { WidgetAssetThumbnail } from "./WidgetAssetThumbnail";

type Facets = NonNullable<AgentRefDTO["facets"]>;
type Bucket = NonNullable<Facets["histogram"]>[number];

const INLINE_PREVIEW_COUNT = 12;
const BOARD_PREVIEW_COUNT = 36;
const INLINE_BUCKETS = 16;
const BOARD_BUCKETS = 28;

/** The timeline widget renders a collection as a draggable "time river":
 * the histogram is the riverbed, the photos *are* the axis. Scrubbing across
 * it lifts the matching moment into the hero panel and (on board) reveals the
 * photos captured in that period. */
export function TimelineWidget({ source, variant, count, title }: WidgetProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const { facets, isLoading, isError } = useWidgetMetadata(source);
  const previewLimit = variant === "board" ? BOARD_PREVIEW_COUNT : INLINE_PREVIEW_COUNT;
  const preview = useWidgetAssetsPreview(source, previewLimit);
  const [carouselAssetId, setCarouselAssetId] = useState<string>();
  const canOpenCarousel = !isMockWidgetSource(source);
  const openAsset = canOpenCarousel ? setCarouselAssetId : undefined;

  const slideIndex = useMemo(
    () =>
      carouselAssetId
        ? preview.assets.findIndex((asset) => asset.asset_id === carouselAssetId)
        : -1,
    [preview.assets, carouselAssetId],
  );

  if (isLoading) return <TimelineSkeleton variant={variant} title={title} />;
  if (isError || !facets?.histogram?.length) {
    return (
      <TimelineShell variant={variant} title={title}>
        <div className="flex h-24 items-center justify-center text-center text-xs text-base-content/50">
          {t("lumilio.widgets.timelineUnavailable", "This timeline is unavailable.")}
        </div>
      </TimelineShell>
    );
  }

  const buckets = facets.histogram;
  const granularity = facets.histogram_granularity ?? inferFacetGranularity(buckets);
  const range = formatRange(facets, granularity, locale);
  // Static t() calls with literal defaults so the extractor keeps these keys
  // and seeds the English values.
  const granularityText = {
    hour: t("lumilio.widgets.timeline.granularity.hour", "By hour"),
    day: t("lumilio.widgets.timeline.granularity.day", "By day"),
    month: t("lumilio.widgets.timeline.granularity.month", "By month"),
    year: t("lumilio.widgets.timeline.granularity.year", "By year"),
  }[granularity];

  return (
    <TimelineShell variant={variant} title={title}>
      <div className="flex items-baseline justify-between gap-3">
        <div className="min-w-0 truncate text-sm font-semibold text-base-content">{range}</div>
        <div className="shrink-0 whitespace-nowrap text-xs text-base-content/45">
          {t("lumilio.widgets.timeline.assets", "{{count}} assets", { count })} · {granularityText}
        </div>
      </div>

      <TimeRiver
        buckets={buckets}
        granularity={granularity}
        total={count}
        variant={variant}
        assets={preview.assets}
        assetsLoading={preview.isLoading}
        onOpen={openAsset}
        source={source}
        locale={locale}
      />

      {carouselAssetId &&
        preview.assets.length > 0 &&
        createPortal(
          <FullScreenCarousel
            photos={preview.assets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={() => setCarouselAssetId(undefined)}
            onNavigate={setCarouselAssetId}
          />,
          document.body,
        )}
    </TimelineShell>
  );
}

function TimelineShell({
  variant,
  title,
  children,
}: {
  variant: WidgetProps["variant"];
  title?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={
        variant === "board"
          ? "h-full overflow-y-auto p-3 space-y-3"
          : "my-3 max-w-2xl rounded-xl border border-base-300 bg-base-100 p-3 space-y-3"
      }
    >
      {variant === "inline" && title && (
        <div className="text-sm font-medium text-base-content/80">{title}</div>
      )}
      {children}
    </div>
  );
}

function TimelineSkeleton({ variant, title }: { variant: WidgetProps["variant"]; title?: string }) {
  return (
    <TimelineShell variant={variant} title={title}>
      <div className="skeleton h-12 rounded-lg" />
      <div className="skeleton h-20 rounded-lg" />
      <div className="skeleton h-24 rounded-lg" />
      {variant === "board" && <div className="skeleton h-36 rounded-lg" />}
    </TimelineShell>
  );
}

/** The scrubbable river: hero panel + interactive density bars. Hovering or
 * dragging across the bars selects a period; on board the selected period's
 * photos appear underneath. */
function TimeRiver({
  buckets,
  granularity,
  total,
  variant,
  assets,
  assetsLoading,
  onOpen,
  source,
  locale,
}: {
  buckets: Bucket[];
  granularity: FacetGranularity;
  total: number;
  variant: WidgetProps["variant"];
  assets: Asset[];
  assetsLoading: boolean;
  onOpen?: (assetId: string) => void;
  source: WidgetProps["source"];
  locale?: string;
}) {
  const { t } = useI18n();
  const maxVisible = variant === "board" ? BOARD_BUCKETS : INLINE_BUCKETS;
  const visible = useMemo(
    () => compressFacetBuckets(buckets, maxVisible),
    [buckets, maxVisible],
  );
  const peak = useMemo(() => Math.max(1, ...buckets.map((b) => b.count ?? 0)), [buckets]);
  const scale = useMemo(() => Math.max(1, ...visible.map((b) => b.count ?? 0)), [visible]);

  // Place the loaded preview assets onto visible buckets by nearest capture
  // time, so the hero and the period grid show real photos from that moment.
  const grouped = useMemo(() => {
    const starts = visible.map((b) => bucketStartDate(b.bucket, granularity)?.getTime() ?? 0);
    const groups: Asset[][] = visible.map(() => []);
    for (const asset of assets) {
      const ts = Date.parse(asset.taken_time ?? asset.upload_time ?? "");
      if (Number.isNaN(ts)) continue;
      let best = 0;
      let bestDelta = Infinity;
      for (let i = 0; i < starts.length; i += 1) {
        const delta = Math.abs(ts - starts[i]);
        if (delta < bestDelta) {
          bestDelta = delta;
          best = i;
        }
      }
      groups[best].push(asset);
    }
    return groups;
  }, [assets, visible, granularity]);

  const defaultIndex = useMemo(() => {
    let index = 0;
    let best = -1;
    visible.forEach((bucket, i) => {
      if ((bucket.count ?? 0) > best) {
        best = bucket.count ?? 0;
        index = i;
      }
    });
    return index;
  }, [visible]);

  const [active, setActive] = useState(defaultIndex);
  useEffect(() => setActive(defaultIndex), [defaultIndex]);

  const trackRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);

  const pickFromPointer = (clientX: number) => {
    const el = trackRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const ratio = Math.min(0.9999, Math.max(0, (clientX - rect.left) / rect.width));
    setActive(Math.floor(ratio * visible.length));
  };

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    dragging.current = true;
    trackRef.current?.setPointerCapture?.(event.pointerId);
    pickFromPointer(event.clientX);
  };
  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.pointerType === "mouse" || dragging.current) pickFromPointer(event.clientX);
  };
  const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    dragging.current = false;
    trackRef.current?.releasePointerCapture?.(event.pointerId);
  };

  const safeActive = Math.min(active, visible.length - 1);
  const activeBucket = visible[safeActive];
  const activeAssets = grouped[safeActive] ?? [];
  const activeCount = activeBucket?.count ?? 0;
  const isPeak = activeCount > 0 && activeCount === peak;
  const sharePct = total > 0 ? Math.round((activeCount / total) * 100) : 0;
  const barHeight = variant === "board" ? "h-24" : "h-16";

  return (
    <div className="space-y-3">
      <HeroMoment
        bucket={activeBucket}
        granularity={granularity}
        rep={activeAssets[0]}
        count={activeCount}
        sharePct={sharePct}
        isPeak={isPeak}
        variant={variant}
        loading={assetsLoading}
        onOpen={onOpen}
        source={source}
        locale={locale}
      />

      <div className="space-y-1.5">
        <div
          ref={trackRef}
          className={`flex ${barHeight} touch-none cursor-ew-resize select-none items-end gap-1`}
          role="slider"
          tabIndex={0}
          aria-valuemin={0}
          aria-valuemax={visible.length - 1}
          aria-valuenow={safeActive}
          aria-label={t("lumilio.widgets.timeline.scrub", "Drag to explore the timeline")}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onKeyDown={(event) => {
            if (event.key === "ArrowRight") setActive((i) => Math.min(visible.length - 1, i + 1));
            if (event.key === "ArrowLeft") setActive((i) => Math.max(0, i - 1));
          }}
        >
          {visible.map((bucket, index) => {
            const value = bucket.count ?? 0;
            const isActive = index === safeActive;
            return (
              <div key={bucket.bucket} className="flex h-full min-w-0 flex-1 items-end">
                <div
                  className={`w-full rounded-t transition-[height,background-color,transform] duration-150 ${
                    isActive ? "bg-primary" : "bg-primary/35 hover:bg-primary/55"
                  }`}
                  style={{
                    height: `${Math.max(8, (value / scale) * 100)}%`,
                    transform: isActive ? "scaleY(1.04)" : undefined,
                    transformOrigin: "bottom",
                  }}
                />
              </div>
            );
          })}
        </div>
        <div className="flex items-center justify-between text-[10px] text-base-content/40">
          <span>{shortTimeBucket(visible[0]?.bucket, granularity, locale)}</span>
          {visible.length > 2 && (
            <span className="flex items-center gap-1">
              <MousePointerClick size={11} />
              {t("lumilio.widgets.timeline.scrubHint", "Drag to explore")}
            </span>
          )}
          <span>{shortTimeBucket(visible[visible.length - 1]?.bucket, granularity, locale)}</span>
        </div>
      </div>

      {variant === "board" && (
        <PeriodGrid
          assets={activeAssets}
          loading={assetsLoading}
          count={activeCount}
          onOpen={onOpen}
          source={source}
        />
      )}
    </div>
  );
}

function HeroMoment({
  bucket,
  granularity,
  rep,
  count,
  sharePct,
  isPeak,
  variant,
  loading,
  onOpen,
  source,
  locale,
}: {
  bucket: Bucket | undefined;
  granularity: FacetGranularity;
  rep: Asset | undefined;
  count: number;
  sharePct: number;
  isPeak: boolean;
  variant: WidgetProps["variant"];
  loading: boolean;
  onOpen?: (assetId: string) => void;
  source: WidgetProps["source"];
  locale?: string;
}) {
  const { t } = useI18n();
  const thumbSize = variant === "board" ? "h-24 w-32" : "h-20 w-28";
  const canOpen = Boolean(onOpen && rep?.asset_id);

  return (
    <div className="flex items-stretch gap-3 rounded-lg border border-base-300 bg-base-200/30 p-2.5">
      <button
        type="button"
        className={`group relative ${thumbSize} shrink-0 overflow-hidden rounded-md bg-base-200`}
        disabled={!canOpen}
        onClick={() => rep?.asset_id && onOpen?.(rep.asset_id)}
      >
        {loading ? (
          <div className="skeleton h-full w-full" />
        ) : rep ? (
          <WidgetAssetThumbnail
            asset={rep}
            source={source}
            className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
          />
        ) : (
          <div className="grid h-full w-full place-items-center text-base-content/30">
            <Images size={20} />
          </div>
        )}
      </button>

      <div className="flex min-w-0 flex-col justify-center gap-1">
        <div className="flex items-center gap-1.5">
          {isPeak && <Flame size={14} className="shrink-0 text-warning" />}
          <span
            className={
              variant === "board"
                ? "truncate text-2xl font-semibold leading-tight text-base-content"
                : "truncate text-lg font-semibold leading-tight text-base-content"
            }
          >
            {formatTimeBucketTitle(bucket?.bucket, granularity, locale)}
          </span>
        </div>
        <div className="truncate text-xs text-base-content/50">
          {t("lumilio.widgets.timeline.assets", "{{count}} assets", { count })}
          {sharePct > 0 && ` · ${sharePct}%`}
        </div>
      </div>
    </div>
  );
}

function PeriodGrid({
  assets,
  loading,
  count,
  onOpen,
  source,
}: {
  assets: Asset[];
  loading: boolean;
  count: number;
  onOpen?: (assetId: string) => void;
  source: WidgetProps["source"];
}) {
  const { t } = useI18n();

  if (loading) {
    return (
      <div className="grid grid-cols-6 gap-1.5">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="skeleton aspect-square rounded-md" />
        ))}
      </div>
    );
  }

  if (assets.length === 0) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-base-content/40">
        <Images size={13} />
        {t("lumilio.widgets.timeline.noPreview", "No preview for this period")}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs font-medium text-base-content/60">
          <Images size={13} />
          {t("lumilio.widgets.timeline.preview", "Preview")}
        </span>
        <span className="text-xs text-base-content/40">{count}</span>
      </div>
      <div className="grid grid-cols-6 gap-1.5">
        {assets.slice(0, 12).map((asset, index) => {
          const canOpen = Boolean(onOpen && asset.asset_id);
          return (
            <button
              key={asset.asset_id ?? index}
              type="button"
              className="group relative aspect-square overflow-hidden rounded-md bg-base-200"
              disabled={!canOpen}
              onClick={() => asset.asset_id && onOpen?.(asset.asset_id)}
            >
              <WidgetAssetThumbnail
                asset={asset}
                source={source}
                className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
              />
            </button>
          );
        })}
      </div>
    </div>
  );
}

function formatRange(facets: Facets, granularity: FacetGranularity, locale?: string): string {
  if (!facets.date_range?.from || !facets.date_range?.to) return "—";
  if (granularity === "hour") {
    const dateFormat = new Intl.DateTimeFormat(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
    const timeFormat = new Intl.DateTimeFormat(locale, {
      hour: "numeric",
      minute: "2-digit",
    });
    const dateTimeFormat = new Intl.DateTimeFormat(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
    const fromDate = new Date(facets.date_range.from);
    const toDate = new Date(facets.date_range.to);
    if (dateFormat.format(fromDate) === dateFormat.format(toDate)) {
      const date = dateFormat.format(fromDate);
      const fromTime = timeFormat.format(fromDate);
      const toTime = timeFormat.format(toDate);
      return fromTime === toTime ? `${date}, ${fromTime}` : `${date}, ${fromTime} - ${toTime}`;
    }
    return `${dateTimeFormat.format(fromDate)} - ${dateTimeFormat.format(toDate)}`;
  }

  const format = new Intl.DateTimeFormat(
    locale,
    granularity === "year"
      ? { year: "numeric" }
      : granularity === "month"
        ? { year: "numeric", month: "short" }
        : { year: "numeric", month: "short", day: "numeric" },
  );
  const from = format.format(new Date(facets.date_range.from));
  const to = format.format(new Date(facets.date_range.to));
  return from === to ? from : `${from} - ${to}`;
}

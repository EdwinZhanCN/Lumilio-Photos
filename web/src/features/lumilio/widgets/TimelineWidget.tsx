import { createPortal } from "react-dom";
import { CalendarDays, Clock, Images } from "lucide-react";
import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { useI18n } from "@/lib/i18n.tsx";
import type { AgentRefDTO } from "../types";
import {
  compressFacetBuckets,
  formatTimeBucketTitle,
  granularityFallbacks,
  inferFacetGranularity,
  shortTimeBucket,
} from "./timeBucketLabels";
import type { FacetGranularity } from "./timeBucketLabels";
import type { WidgetProps } from "./types";
import { useWidgetAssetsPreview } from "./useWidgetAssets";
import { useWidgetMetadata } from "./useWidgetMetadata";

type Facets = NonNullable<AgentRefDTO["facets"]>;
type Bucket = NonNullable<Facets["histogram"]>[number];

const INLINE_PREVIEW_COUNT = 6;
const BOARD_PREVIEW_COUNT = 12;

export function TimelineWidget({ source, variant, count, title }: WidgetProps) {
  const { t } = useI18n();
  const { facets, isLoading, isError } = useWidgetMetadata(source);
  const previewLimit = variant === "board" ? BOARD_PREVIEW_COUNT : INLINE_PREVIEW_COUNT;
  const preview = useWidgetAssetsPreview(source, previewLimit);
  const [carouselAssetId, setCarouselAssetId] = useState<string>();

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
  const range = formatRange(facets, granularity);
  const granularityText = t(
    `lumilio.widgets.timeline.granularity.${granularity}`,
    granularityFallbacks[granularity],
  );

  return (
    <TimelineShell variant={variant} title={title}>
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-xs font-medium text-base-content/60">
            <CalendarDays size={14} />
            <span>{t("lumilio.widgets.timeline.range", "Range")}</span>
          </div>
          <div className="mt-1 truncate text-sm font-semibold text-base-content">{range}</div>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          <span className="badge badge-outline badge-sm">
            {t("lumilio.widgets.timeline.assets", "{{count}} assets", {
              count,
            })}
          </span>
          <span className="badge badge-ghost badge-sm">{granularityText}</span>
        </div>
      </div>

      {variant === "inline" ? (
        <>
          <PhotoStrip
            assets={preview.assets}
            isLoading={preview.isLoading}
            variant="inline"
            onOpen={setCarouselAssetId}
          />
          <InlineTimeline buckets={buckets} granularity={granularity} />
        </>
      ) : (
        <BoardTimeline
          buckets={buckets}
          granularity={granularity}
          assets={preview.assets}
          assetsLoading={preview.isLoading}
          count={count}
          onOpen={setCarouselAssetId}
        />
      )}

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
      <div className="skeleton h-28 rounded-lg" />
      {variant === "board" && <div className="skeleton h-36 rounded-lg" />}
    </TimelineShell>
  );
}

function InlineTimeline({
  buckets,
  granularity,
}: {
  buckets: Bucket[];
  granularity: FacetGranularity;
}) {
  const visible = useMemo(() => compressFacetBuckets(buckets, 10), [buckets]);
  const max = Math.max(1, ...visible.map((bucket) => bucket.count ?? 0));

  return (
    <div className="space-y-2">
      <div className="flex h-16 gap-1.5">
        {visible.map((bucket) => (
          <div key={bucket.bucket} className="flex h-full min-w-0 flex-1 flex-col gap-1">
            <div className="flex min-h-0 flex-1 items-end">
              <div
                className="w-full rounded-t bg-primary/60"
                style={{
                  height: `${Math.max(10, ((bucket.count ?? 0) / max) * 100)}%`,
                }}
                title={`${formatTimeBucketTitle(bucket.bucket, granularity)}: ${bucket.count}`}
              />
            </div>
            <span className="w-full truncate text-center text-[10px] text-base-content/45">
              {shortTimeBucket(bucket.bucket, granularity)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function BoardTimeline({
  buckets,
  granularity,
  assets,
  assetsLoading,
  count,
  onOpen,
}: {
  buckets: Bucket[];
  granularity: FacetGranularity;
  assets: Asset[];
  assetsLoading: boolean;
  count: number;
  onOpen: (assetId: string) => void;
}) {
  const visible = useMemo(() => compressFacetBuckets(buckets, 18), [buckets]);

  if (buckets.length === 1) {
    return (
      <SinglePeriodFocus
        bucket={buckets[0]}
        granularity={granularity}
        assets={assets}
        assetsLoading={assetsLoading}
        count={count}
        onOpen={onOpen}
      />
    );
  }

  return (
    <div className="space-y-3">
      <PhotoStrip assets={assets} isLoading={assetsLoading} variant="board" onOpen={onOpen} />
      <RhythmRail buckets={visible} granularity={granularity} />
    </div>
  );
}

function SinglePeriodFocus({
  bucket,
  granularity,
  assets,
  assetsLoading,
  count,
  onOpen,
}: {
  bucket: Bucket;
  granularity: FacetGranularity;
  assets: Asset[];
  assetsLoading: boolean;
  count: number;
  onOpen: (assetId: string) => void;
}) {
  const { t } = useI18n();

  return (
    <div className="space-y-3">
      <FocusMosaic assets={assets} isLoading={assetsLoading} onOpen={onOpen} />
      <div className="space-y-2">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2 text-xs font-medium text-base-content/55">
              <Clock size={14} />
              <span>{t("lumilio.widgets.timeline.focusedPeriod", "Focused period")}</span>
            </div>
            <div className="truncate text-3xl font-semibold text-base-content">
              {formatTimeBucketTitle(bucket.bucket, granularity)}
            </div>
            <div className="text-xs text-base-content/45">{bucket.bucket}</div>
          </div>
          <span className="badge badge-primary badge-sm shrink-0">
            {t("lumilio.widgets.timeline.assets", "{{count}} assets", {
              count,
            })}
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-base-300">
          <div className="h-full w-full rounded-full bg-primary/70" />
        </div>
      </div>
    </div>
  );
}

function FocusMosaic({
  assets,
  isLoading,
  onOpen,
}: {
  assets: Asset[];
  isLoading: boolean;
  onOpen: (assetId: string) => void;
}) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-4 grid-rows-2 gap-1.5">
        <div className="skeleton col-span-2 row-span-2 aspect-square rounded-lg" />
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={index} className="skeleton aspect-square rounded-lg" />
        ))}
      </div>
    );
  }

  if (assets.length === 0) return null;

  return (
    <div className="grid grid-cols-4 grid-rows-2 gap-1.5">
      {assets.slice(0, 5).map((asset, index) => (
        <button
          key={asset.asset_id ?? index}
          className={
            index === 0
              ? "group relative col-span-2 row-span-2 aspect-square overflow-hidden rounded-lg bg-base-200"
              : "group relative aspect-square overflow-hidden rounded-lg bg-base-200"
          }
          onClick={() => asset.asset_id && onOpen(asset.asset_id)}
        >
          <img
            src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
            alt=""
            loading="lazy"
            className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
          />
        </button>
      ))}
    </div>
  );
}

function RhythmRail({
  buckets,
  granularity,
}: {
  buckets: Bucket[];
  granularity: FacetGranularity;
}) {
  const { t } = useI18n();
  const max = Math.max(1, ...buckets.map((bucket) => bucket.count ?? 0));

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-xs font-medium text-base-content/55">
        <CalendarDays size={14} />
        <span>{t("lumilio.widgets.timeline.rhythm", "Rhythm")}</span>
      </div>
      <div className="flex h-14 gap-1">
        {buckets.map((bucket) => (
          <div key={bucket.bucket} className="flex h-full min-w-0 flex-1 flex-col gap-1">
            <div className="flex min-h-0 flex-1 items-end">
              <div
                className="w-full rounded-t bg-primary/65"
                style={{
                  height: `${Math.max(9, ((bucket.count ?? 0) / max) * 100)}%`,
                }}
                title={`${formatTimeBucketTitle(bucket.bucket, granularity)}: ${bucket.count}`}
              />
            </div>
            <span className="truncate text-center text-[10px] text-base-content/40">
              {shortTimeBucket(bucket.bucket, granularity)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function PhotoStrip({
  assets,
  isLoading,
  variant,
  onOpen,
}: {
  assets: Asset[];
  isLoading: boolean;
  variant: WidgetProps["variant"];
  onOpen: (assetId: string) => void;
}) {
  const { t } = useI18n();
  const skeletonCount = variant === "board" ? 8 : 6;

  if (isLoading) {
    return (
      <div className="grid grid-cols-4 gap-1.5">
        {Array.from({ length: skeletonCount }).map((_, index) => (
          <div key={index} className="skeleton aspect-square rounded-lg" />
        ))}
      </div>
    );
  }

  if (assets.length === 0) return null;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs font-medium text-base-content/60">
          <Images size={13} />
          {t("lumilio.widgets.timeline.preview", "Preview")}
        </span>
        <span className="text-xs text-base-content/40">{assets.length}</span>
      </div>
      <div
        className={variant === "board" ? "grid grid-cols-4 gap-1.5" : "grid grid-cols-6 gap-1.5"}
      >
        {assets.map((asset, index) => (
          <button
            key={asset.asset_id ?? index}
            className={
              variant === "board" && index === 0
                ? "group relative col-span-2 row-span-2 aspect-square overflow-hidden rounded-lg bg-base-200"
                : "group relative aspect-square overflow-hidden rounded-lg bg-base-200"
            }
            onClick={() => asset.asset_id && onOpen(asset.asset_id)}
          >
            <img
              src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
              alt=""
              loading="lazy"
              className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
            />
          </button>
        ))}
      </div>
    </div>
  );
}

function formatRange(facets: Facets, granularity: FacetGranularity): string {
  if (!facets.date_range?.from || !facets.date_range?.to) return "—";
  if (granularity === "hour") {
    const dateFormat = new Intl.DateTimeFormat(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
    const timeFormat = new Intl.DateTimeFormat(undefined, {
      hour: "numeric",
      minute: "2-digit",
    });
    const dateTimeFormat = new Intl.DateTimeFormat(undefined, {
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
    undefined,
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

import { BarChart3, CalendarDays, Camera, Heart, Image, MapPin, Star, Users } from "lucide-react";
import { useMemo } from "react";
import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import type { AgentRefDTO } from "../types";
import {
  compressFacetBuckets,
  formatTimeBucketTitle,
  inferFacetGranularity,
  shortTimeBucket,
} from "./timeBucketLabels";
import type { WidgetProps } from "./types";
import { useWidgetMetadata } from "./useWidgetMetadata";

type Facets = NonNullable<AgentRefDTO["facets"]>;
type NameCount = NonNullable<Facets["top_places"]>[number];

const compactNumber = new Intl.NumberFormat(undefined, {
  notation: "compact",
  maximumFractionDigits: 1,
});

export function FacetDashboardWidget({ source, variant, count, title }: WidgetProps) {
  const { t } = useI18n();
  const { facets, isLoading, isError } = useWidgetMetadata(source);

  if (isLoading) return <WidgetSkeleton variant={variant} />;
  if (isError || !facets) {
    return (
      <WidgetShell variant={variant} title={title}>
        <div className="flex h-24 items-center justify-center text-center text-xs text-base-content/50">
          {t("lumilio.widgets.metadataUnavailable", "This widget's summary is unavailable.")}
        </div>
      </WidgetShell>
    );
  }

  const mediaTypes = facets.types ?? {};
  const ratingDist = facets.rating_dist ?? [];
  const ratedCount = ratingDist.reduce((sum, value, index) => sum + (index > 0 ? value : 0), 0);

  return (
    <WidgetShell variant={variant} title={title}>
      <div className="grid grid-cols-2 gap-2">
        <Metric
          icon={<Image size={16} />}
          label={t("lumilio.widgets.metrics.assets", "Assets")}
          value={count}
        />
        <Metric
          icon={<Heart size={16} />}
          label={t("lumilio.widgets.metrics.liked", "Liked")}
          value={facets.liked_count ?? 0}
        />
        <Metric
          icon={<CalendarDays size={16} />}
          label={t("lumilio.widgets.metrics.range", "Range")}
          value={formatDateRange(facets)}
          compact
        />
        <Metric
          icon={<Star size={16} />}
          label={t("lumilio.widgets.metrics.rated", "Rated")}
          value={ratedCount}
        />
      </div>

      <Histogram facets={facets} />

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <FacetList
          icon={<MapPin size={14} />}
          title={t("lumilio.widgets.facets.places", "Places")}
          values={facets.top_places}
        />
        <FacetList
          icon={<Users size={14} />}
          title={t("lumilio.widgets.facets.people", "People")}
          values={facets.top_people}
        />
        <FacetList
          icon={<Camera size={14} />}
          title={t("lumilio.widgets.facets.cameras", "Cameras")}
          values={facets.cameras}
        />
        <TypeBreakdown values={mediaTypes} />
      </div>
    </WidgetShell>
  );
}

function WidgetShell({
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

function WidgetSkeleton({ variant }: { variant: WidgetProps["variant"] }) {
  return (
    <WidgetShell variant={variant}>
      <div className="grid grid-cols-2 gap-2">
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={index} className="skeleton h-14 rounded-lg" />
        ))}
      </div>
      <div className="skeleton h-20 rounded-lg" />
      <div className="grid grid-cols-2 gap-3">
        <div className="skeleton h-24 rounded-lg" />
        <div className="skeleton h-24 rounded-lg" />
      </div>
    </WidgetShell>
  );
}

function Metric({
  icon,
  label,
  value,
  compact = false,
}: {
  icon: ReactNode;
  label: string;
  value: number | string;
  compact?: boolean;
}) {
  const displayValue = typeof value === "number" ? compactNumber.format(value) : value;
  return (
    <div className="rounded-lg border border-base-300 bg-base-200/35 px-3 py-2">
      <div className="flex items-center gap-2 text-xs text-base-content/55">
        {icon}
        <span className="truncate">{label}</span>
      </div>
      <div
        className={
          compact
            ? "mt-1 truncate text-sm font-semibold text-base-content"
            : "mt-1 text-lg font-semibold text-base-content"
        }
      >
        {displayValue}
      </div>
    </div>
  );
}

function Histogram({ facets }: { facets: Facets }) {
  const { t } = useI18n();
  const rawBuckets = useMemo(() => facets.histogram ?? [], [facets.histogram]);
  const granularity = facets.histogram_granularity ?? inferFacetGranularity(rawBuckets);
  const buckets = useMemo(() => compressFacetBuckets(rawBuckets, 14), [rawBuckets]);
  const max = Math.max(1, ...buckets.map((bucket) => bucket.count ?? 0));

  if (buckets.length === 0) return null;

  return (
    <div className="rounded-lg border border-base-300 bg-base-100 p-3">
      <div className="mb-2 flex items-center gap-2 text-xs font-medium text-base-content/65">
        <BarChart3 size={14} />
        <span>{t("lumilio.widgets.facets.timeline", "Timeline")}</span>
      </div>
      <div className="flex h-24 gap-1">
        {buckets.map((bucket) => (
          <div key={bucket.bucket} className="flex h-full min-w-0 flex-1 flex-col gap-1">
            <div className="flex min-h-0 flex-1 items-end">
              <div
                className="w-full rounded-t bg-primary/55"
                style={{
                  height: `${Math.max(8, ((bucket.count ?? 0) / max) * 100)}%`,
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

function FacetList({
  icon,
  title,
  values,
}: {
  icon: ReactNode;
  title: string;
  values?: NameCount[];
}) {
  const topValues = (values ?? []).slice(0, 4);
  if (topValues.length === 0) return null;
  const max = Math.max(1, ...topValues.map((value) => value.count ?? 0));

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-xs font-medium text-base-content/65">
        {icon}
        <span>{title}</span>
      </div>
      <div className="space-y-1.5">
        {topValues.map((value) => (
          <div key={value.name} className="space-y-1">
            <div className="flex items-center justify-between gap-2 text-xs">
              <span className="truncate text-base-content/75">{value.name}</span>
              <span className="shrink-0 text-base-content/45">{value.count}</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-base-300">
              <div
                className="h-full rounded-full bg-secondary/60"
                style={{
                  width: `${Math.max(8, ((value.count ?? 0) / max) * 100)}%`,
                }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function TypeBreakdown({ values }: { values: Record<string, number> }) {
  const { t } = useI18n();
  const entries = Object.entries(values).filter(([, value]) => value > 0);
  if (entries.length === 0) return null;

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-xs font-medium text-base-content/65">
        <Image size={14} />
        <span>{t("lumilio.widgets.facets.types", "Types")}</span>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {entries.map(([name, value]) => (
          <span key={name} className="badge badge-outline badge-sm">
            {name.toLowerCase()} {value}
          </span>
        ))}
      </div>
    </div>
  );
}

function formatDateRange(facets: Facets): string {
  if (!facets.date_range?.from || !facets.date_range?.to) return "—";
  const granularity = facets.histogram_granularity ?? inferFacetGranularity(facets.histogram ?? []);
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
    const fromDate = new Date(facets.date_range.from);
    const toDate = new Date(facets.date_range.to);
    if (dateFormat.format(fromDate) === dateFormat.format(toDate)) {
      return `${dateFormat.format(fromDate)}, ${timeFormat.format(fromDate)} - ${timeFormat.format(toDate)}`;
    }
  }
  const options: Intl.DateTimeFormatOptions =
    granularity === "year"
      ? { year: "numeric" }
      : granularity === "month"
        ? { year: "numeric", month: "short" }
        : { year: "numeric", month: "short", day: "numeric" };
  const format = new Intl.DateTimeFormat(undefined, options);
  const from = format.format(new Date(facets.date_range.from));
  const to = format.format(new Date(facets.date_range.to));
  return from === to ? from : `${from} - ${to}`;
}

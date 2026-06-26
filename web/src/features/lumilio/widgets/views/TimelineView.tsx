import { Calendar } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { dateRangeLabel, fmt } from "../format";
import type { AgentRefFacetsDTO, ViewBodyProps } from "../types";

type Bucket = NonNullable<AgentRefFacetsDTO["histogram"]>[number];

/** Timeline — distribution over time, drawn as a hand-rolled div bar chart (no
 * chart lib). L gains a header (date span, count, granularity) and per-bar
 * hover counts; x-axis labels thin out as the tier shrinks. */
export function TimelineView({ data, size }: ViewBodyProps) {
  const { t } = useI18n();
  const { i18n } = useI18n();
  const locale = i18n.language;
  const facets = data.facets;
  const buckets: Bucket[] = facets?.histogram ?? [];
  const granularity = facets?.histogram_granularity;
  const showHead = size === "l";
  const showLabels = size !== "s";

  const granularityLabel = (g: string): string => {
    switch (g) {
      case "hour":
        return t("lumilio.widgets.timeline.byHour", "by hour");
      case "day":
        return t("lumilio.widgets.timeline.byDay", "by day");
      case "month":
        return t("lumilio.widgets.timeline.byMonth", "by month");
      case "year":
        return t("lumilio.widgets.timeline.byYear", "by year");
      default:
        return g;
    }
  };

  const pad = size === "s" ? "p-2 pt-2.5" : "p-3";

  if (buckets.length === 0) {
    return (
      <div className={`flex h-full flex-col items-center justify-center ${pad}`}>
        <span className="text-xs text-base-content/45">
          {t("lumilio.widgets.timeline.noData", "No timeline data")}
        </span>
      </div>
    );
  }

  const max = Math.max(1, ...buckets.map((b) => b.count ?? 0));
  const labelBuckets =
    size === "l"
      ? buckets.filter((_, i) => i % 2 === 0)
      : [buckets[0], buckets[Math.floor(buckets.length / 2)], buckets[buckets.length - 1]];

  return (
    <div className={`flex h-full flex-col ${pad}`}>
      {showHead && (
        <div className="mb-2 flex shrink-0 items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-base-content/55">
            {dateRangeLabel(facets?.date_range) && (
              <span className="inline-flex items-center gap-1">
                <Calendar size={14} strokeWidth={1.75} />
                {dateRangeLabel(facets?.date_range)}
              </span>
            )}
            <span className="font-semibold tabular-nums text-base-content/75">
              {fmt(data.count, locale)} {t("lumilio.widgets.photos", "photos")}
            </span>
          </div>
          {granularity && (
            <span className="badge badge-ghost badge-sm">{granularityLabel(granularity)}</span>
          )}
        </div>
      )}
      <div className="flex min-h-0 flex-1 items-end border-b border-base-content/10">
        <div className="flex h-full w-full items-end gap-[3px]">
          {buckets.map((b, i) => {
            const pct = Math.max(6, Math.round(((b.count ?? 0) / max) * 100));
            return (
              <div
                key={`${b.bucket}-${i}`}
                className="group/bar relative flex h-full min-w-0 flex-1 flex-col justify-end"
              >
                {showHead && (
                  <div className="mb-0.5 text-center text-[10px] font-semibold tabular-nums text-base-content/70 opacity-0 transition-opacity group-hover/bar:opacity-100">
                    {fmt(b.count, locale)}
                  </div>
                )}
                <div
                  className="w-full rounded-t-[3px] bg-primary/45 transition-colors group-hover/bar:bg-primary"
                  style={{ height: `${pct}%` }}
                  title={`${b.bucket}: ${b.count}`}
                />
              </div>
            );
          })}
        </div>
      </div>
      {showLabels && (
        <div className="mt-1 flex shrink-0 justify-between text-[10px] font-medium text-base-content/40">
          {labelBuckets.map((b, i) => (
            <span key={i} className="tabular-nums">
              {b ? b.bucket : ""}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

import React from "react";

type HistogramDatum = {
  date: string | Date;
  count?: number;
  [key: string]: unknown;
};

type TooltipValue = { date: string; count: number };

export type HistogramProps = {
  /**
   * Same values used by CalendarHeatmap. Duplicated dates are summed.
   */
  values: HistogramDatum[];
  /**
   * Optional range. If omitted, the range is derived from `values`.
   */
  startDate?: string | Date;
  endDate?: string | Date;
  /**
   * Component outer className. Apply text color here to color the bars
   * (bars use fill: currentColor by default).
   */
  className?: string;
  /**
   * Inline styles for the outer wrapper.
   */
  style?: React.CSSProperties;
  /**
   * Fixed pixel height of the histogram area (responsive width).
   * Default: 96
   */
  height?: number;
  /**
   * Bar color. If omitted, uses currentColor (so you can control via className).
   */
  barColor?: string;
  /**
   * Bar corner radius (viewBox units). Default: 0.2
   */
  barRadius?: number;
  /**
   * Gap between bars in viewBox units (each day is 1 unit wide). Default: 0.2
   */
  barGap?: number;
  /**
   * Provide arbitrary attributes to attach on each bar (e.g. data-tip).
   */
  tooltipDataAttrs?: (
    value?: TooltipValue,
  ) => Record<string, string | number | undefined> | undefined;
  /**
   * Called when a bar is clicked.
   */
  onBarClick?: (value: TooltipValue, index: number) => void;
  /**
   * Custom className for each bar based on its value.
   */
  classForBar?: (value?: TooltipValue) => string;
};

/**
 * A simple, dependency-free, responsive histogram meant to sit under
 * the CalendarHeatmap and consume the same data array.
 *
 * - Uses an SVG with a responsive viewBox (100% width).
 * - One "day" equals 1 viewBox unit on the X axis.
 * - Heights are normalized to a 0..100 Y range.
 */
const Histogram: React.FC<HistogramProps> = ({
  values,
  startDate,
  endDate,
  className,
  style,
  height = 96,
  barColor,
  barRadius = 0.2,
  barGap = 0.2,
  tooltipDataAttrs,
  onBarClick,
  classForBar,
}) => {
  // Helpers
  const parseDate = (d: string | Date): Date =>
    d instanceof Date ? new Date(d) : new Date(d);
  const dateKey = (d: Date): string => d.toISOString().slice(0, 10); // YYYY-MM-DD

  const startOfDay = (d: Date) => {
    const x = new Date(d);
    x.setHours(0, 0, 0, 0);
    return x;
  };

  // Merge duplicate dates and normalize input
  const valueMap = React.useMemo(() => {
    const map = new Map<string, number>();
    for (const v of values || []) {
      const d = parseDate(v.date);
      if (Number.isNaN(d.getTime())) continue;
      const key = dateKey(d);
      const count = typeof v.count === "number" ? v.count : 0;
      map.set(key, (map.get(key) || 0) + count);
    }
    return map;
  }, [values]);

  // Determine range from props or data (fallback to last 30 days if empty)
  const [rangeStart, rangeEnd] = React.useMemo(() => {
    let s: Date | null = startDate ? startOfDay(parseDate(startDate)) : null;
    let e: Date | null = endDate ? startOfDay(parseDate(endDate)) : null;

    if (!s || !e) {
      if (valueMap.size > 0) {
        const allDates = Array.from(valueMap.keys()).map((k) =>
          startOfDay(new Date(k)),
        );
        allDates.sort((a, b) => a.getTime() - b.getTime());
        s = s || allDates[0];
        e = e || allDates[allDates.length - 1];
      } else {
        // Fallback: last 30 days ending today
        const today = startOfDay(new Date());
        const start = new Date(today);
        start.setDate(today.getDate() - 29);
        s = s || start;
        e = e || today;
      }
    }

    return [s!, e!] as const;
  }, [startDate, endDate, valueMap]);

  // Build list of days from start to end inclusive
  const days = React.useMemo(() => {
    const list: string[] = [];
    const cursor = new Date(rangeStart);
    while (cursor.getTime() <= rangeEnd.getTime()) {
      list.push(dateKey(cursor));
      cursor.setDate(cursor.getDate() + 1);
    }
    return list;
  }, [rangeStart, rangeEnd]);

  // Compute counts aligned to range and determine max
  const counts = React.useMemo(() => {
    return days.map((key) => valueMap.get(key) ?? 0);
  }, [days, valueMap]);

  const maxCount = React.useMemo(() => {
    let m = 0;
    for (const c of counts) if (c > m) m = c;
    return m || 1; // avoid division by zero
  }, [counts]);

  // Layout in viewBox units: width = number of days, height = 100
  const vbWidth = Math.max(days.length, 1);
  const vbHeight = 100;
  const unitWidth = 1; // per day
  const gap = Math.max(0, Math.min(barGap, unitWidth)); // clamp gap
  const barW = Math.max(0, unitWidth - gap);

  return (
    <div
      className={className}
      style={{
        width: "100%",
        height,
        display: "block",
        ...style,
      }}
    >
      <svg
        viewBox={`0 0 ${vbWidth} ${vbHeight}`}
        preserveAspectRatio="xMinYMin meet"
        width="100%"
        height="100%"
        role="img"
        aria-label="Histogram"
      >
        <g>
          {counts.map((c, i) => {
            const h = Math.round((c / maxCount) * vbHeight * 1000) / 1000; // round to 0.001
            const x = i * unitWidth + gap / 2;
            const y = vbHeight - h;
            const key = days[i];
            const value = { date: key, count: c };
            const attrs = ((tooltipDataAttrs
              ? tooltipDataAttrs(value)
              : undefined) ?? {}) as Record<
              string,
              string | number | undefined
            >;
            const title =
              (attrs && (attrs as any).title) ||
              `${key} • ${c} ${c === 1 ? "item" : "items"}`;
            const barClass = classForBar ? classForBar(value) : "";

            return (
              <g key={key} transform="" className={barClass}>
                <rect
                  x={x}
                  y={y}
                  width={barW}
                  height={h}
                  rx={barRadius}
                  ry={barRadius}
                  fill={barColor || "currentColor"}
                  data-date={key}
                  {...attrs}
                  onClick={onBarClick ? () => onBarClick(value, i) : undefined}
                >
                  <title>{title}</title>
                </rect>
              </g>
            );
          })}
        </g>
      </svg>
    </div>
  );
};

export default Histogram;

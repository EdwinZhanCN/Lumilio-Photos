import React from "react";

type HeatmapDatum = {
  date: string | Date;
  count?: number;
  // Allow passing through any extra info (e.g. for tooltips)
  [key: string]: unknown;
};

type CalendarHeatmapProps = {
  values: HeatmapDatum[];
  // Custom class name for each day's rect based on its value
  classForValue?: (value?: { date: string; count: number }) => string;
  // Provide arbitrary attributes to attach on each rect (e.g. data-tip for DaisyUI)
  tooltipDataAttrs?: (value?: {
    date: string;
    count: number;
  }) => Record<string, string | number | undefined> | undefined;
  // Optional range override. If not provided, range is derived from `values`
  startDate?: string | Date;
  endDate?: string | Date;
  // Layout
  rectSize?: number; // square size in px
  gutter?: number; // gap between squares in px
  weekStart?: 0 | 1 | 2 | 3 | 4 | 5 | 6; // 0 = Sunday (default)
  showMonthLabels?: boolean;
  className?: string;
  style?: React.CSSProperties;
};

/**
 * Minimal, dependency-free calendar heatmap (GitHub-style).
 * - Uses SVG rectangles arranged by weeks (columns) and weekdays (rows).
 * - Compatible with existing `.react-calendar-heatmap .color-*` styles in heatmap.css
 */
const CalendarHeatmap: React.FC<CalendarHeatmapProps> = ({
  values,
  classForValue,
  tooltipDataAttrs,
  startDate,
  endDate,
  rectSize = 12,
  gutter = 3,
  weekStart = 0,
  showMonthLabels = true,
  className,
  style,
}) => {
  // Helpers
  const parseDate = (d: string | Date): Date =>
    d instanceof Date ? new Date(d) : new Date(d);
  const dateKey = (d: Date): string => d.toISOString().slice(0, 10); // YYYY-MM-DD
  const clamp = (n: number, min: number, max: number) =>
    Math.min(max, Math.max(min, n));

  const startOfWeek = (d: Date, ws: number) => {
    const res = new Date(d);
    while (res.getDay() !== ws) {
      res.setDate(res.getDate() - 1);
    }
    res.setHours(0, 0, 0, 0);
    return res;
  };

  const endOfWeek = (d: Date, ws: number) => {
    const endDay = (ws + 6) % 7;
    const res = new Date(d);
    while (res.getDay() !== endDay) {
      res.setDate(res.getDate() + 1);
    }
    res.setHours(0, 0, 0, 0);
    return res;
  };

  const monthShort = (idx: number) =>
    [
      "Jan",
      "Feb",
      "Mar",
      "Apr",
      "May",
      "Jun",
      "Jul",
      "Aug",
      "Sep",
      "Oct",
      "Nov",
      "Dec",
    ][idx];

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

  // Determine range
  const [rangeStart, rangeEnd] = React.useMemo(() => {
    let s: Date | null = null;
    let e: Date | null = null;

    if (startDate) s = parseDate(startDate);
    if (endDate) e = parseDate(endDate);

    if (!s || !e) {
      // derive from values; fallback to current year
      if (valueMap.size > 0) {
        const allDates = Array.from(valueMap.keys()).map((k) => new Date(k));
        allDates.sort((a, b) => a.getTime() - b.getTime());
        s = s || allDates[0];
        e = e || allDates[allDates.length - 1];
      } else {
        const now = new Date();
        const year = now.getFullYear();
        s = s || new Date(year, 0, 1);
        e = e || new Date(year, 11, 31);
      }
    }

    // Expand to full weeks
    const startAligned = startOfWeek(s!, weekStart);
    const endAligned = endOfWeek(e!, weekStart);
    return [startAligned, endAligned] as const;
  }, [startDate, endDate, valueMap, weekStart]);

  // Build days across the range
  const days: Date[] = React.useMemo(() => {
    const list: Date[] = [];
    const cursor = new Date(rangeStart);
    while (cursor.getTime() <= rangeEnd.getTime()) {
      list.push(new Date(cursor));
      cursor.setDate(cursor.getDate() + 1);
    }
    return list;
  }, [rangeStart, rangeEnd]);

  // Group into weeks (columns)
  const weeks = React.useMemo(() => {
    const out: Date[][] = [];
    let current: Date[] = [];
    for (let i = 0; i < days.length; i++) {
      const d = days[i];
      if (d.getDay() === weekStart && current.length > 0) {
        out.push(current);
        current = [];
      }
      current.push(d);
    }
    if (current.length > 0) out.push(current);
    // Ensure each week has 7 entries (range is aligned so it should)
    return out;
  }, [days, weekStart]);

  // Size and layout
  const cols = weeks.length;
  const rows = 7;
  const topPad = showMonthLabels ? 16 : 0;
  const svgWidth = cols * rectSize + (cols - 1) * gutter;
  const svgHeight = topPad + rows * rectSize + (rows - 1) * gutter;

  const defaultClassForValue = (v?: { date: string; count: number }) => {
    if (!v || !v.count) return "color-empty";
    return `color-scale-${clamp(v.count, 0, 5)}`;
  };

  // Month labels positions (first day of month within each week)
  const monthLabels = React.useMemo(() => {
    if (!showMonthLabels) return [];
    const labels: { x: number; text: string }[] = [];
    weeks.forEach((week, weekIdx) => {
      const firstOfMonth = week.find((d) => d.getDate() === 1);
      if (firstOfMonth) {
        const x = weekIdx * (rectSize + gutter);
        labels.push({ x, text: monthShort(firstOfMonth.getMonth()) });
      }
    });
    return labels;
  }, [weeks, showMonthLabels, rectSize, gutter]);

  return (
    <div
      className={`react-calendar-heatmap ${className ?? ""}`}
      style={{ display: "inline-block", ...style }}
    >
      <svg
        width={svgWidth}
        height={svgHeight}
        role="img"
        aria-label="Calendar heatmap"
      >
        {showMonthLabels && monthLabels.length > 0 && (
          <g transform={`translate(0, ${topPad - 4})`} aria-hidden="true">
            {monthLabels.map((m, i) => (
              <text
                key={`${m.text}-${i}`}
                x={m.x}
                y={0}
                fontSize={10}
                fill="currentColor"
              >
                {m.text}
              </text>
            ))}
          </g>
        )}

        <g transform={`translate(0, ${topPad})`}>
          {weeks.map((week, weekIdx) => {
            const x = weekIdx * (rectSize + gutter);
            return (
              <g key={weekIdx} transform={`translate(${x}, 0)`}>
                {week.map((day, dayIdx) => {
                  const y = dayIdx * (rectSize + gutter);
                  const key = dateKey(day);
                  const count = valueMap.get(key) ?? 0;
                  const value = { date: key, count };
                  const dayClass =
                    (classForValue
                      ? classForValue(count > 0 ? value : undefined)
                      : defaultClassForValue(count > 0 ? value : undefined)) ||
                    "";
                  const dataAttrs = ((tooltipDataAttrs
                    ? tooltipDataAttrs(count > 0 ? value : undefined)
                    : undefined) ?? {}) as Record<
                    string,
                    string | number | undefined
                  >;
                  const title =
                    (dataAttrs && (dataAttrs as any).title) ||
                    `${key} • ${count} ${count === 1 ? "item" : "items"}`;

                  return (
                    <rect
                      key={key}
                      x={0}
                      y={y}
                      width={rectSize}
                      height={rectSize}
                      rx={2}
                      ry={2}
                      className={dayClass}
                      data-date={key}
                      {...dataAttrs}
                    >
                      <title>{title}</title>
                    </rect>
                  );
                })}
              </g>
            );
          })}
        </g>
      </svg>
    </div>
  );
};

export default CalendarHeatmap;

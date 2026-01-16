import React, { useMemo } from "react";

export interface HeatmapValue {
  date: string | Date;
  count: number;
}

export interface GitHubStyleHeatmapProps {
  values: HeatmapValue[];
  year?: number; // 指定显示的年份
  showMonthLabels?: boolean;
  showWeekdayLabels?: boolean;
  cellSize?: number;
  cellGap?: number;
  className?: string;
  emptyColor?: string;
  colors?: string[];
}

interface DayData {
  date: Date;
  count: number;
  level: number;
  inRange: boolean;
}

const WEEKDAY_LABELS = ["", "Mon", "", "Wed", "", "Fri", ""];
const MONTH_LABELS = [
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
];

// GitHub 默认使用的绿色系配色
const DEFAULT_COLORS = [
  "fill-green-200 bg-green-200",
  "fill-green-300 bg-green-300",
  "fill-green-500 bg-green-500",
  "fill-green-700 bg-green-700",
];

const DEFAULT_EMPTY_COLOR = "fill-base-200 bg-base-200";

/**
 * GitHub Style Contribution Heatmap
 * 精确模仿 GitHub 的贡献图样式，支持按年份显示
 */
export const GitHubStyleHeatmap: React.FC<GitHubStyleHeatmapProps> = ({
  values,
  year,
  showMonthLabels = true,
  showWeekdayLabels = true,
  cellSize = 11,
  cellGap = 3,
  className = "",
  emptyColor = DEFAULT_EMPTY_COLOR,
  colors = DEFAULT_COLORS,
}) => {
  // 解析日期
  const parseDate = (date: string | Date): Date => {
    return typeof date === "string" ? new Date(date) : new Date(date);
  };

  // 格式化日期为 YYYY-MM-DD
  const formatDate = (date: Date): string => {
    return date.toISOString().split("T")[0];
  };

  // 获取指定日期所在周的周日
  const getWeekStart = (date: Date): Date => {
    const d = new Date(date);
    const day = d.getDay();
    d.setDate(d.getDate() - day);
    d.setHours(0, 0, 0, 0);
    return d;
  };

  // 创建值映射
  const valueMap = useMemo(() => {
    const map = new Map<string, number>();
    values.forEach((v) => {
      const date = parseDate(v.date);
      const key = formatDate(date);
      map.set(key, (map.get(key) || 0) + v.count);
    });
    return map;
  }, [values]);

  // 计算日期范围（基于指定年份或当前年份）
  const { start, end, yearStart, yearEnd } = useMemo(() => {
    const targetYear = year || new Date().getFullYear();

    const yearStart = new Date(targetYear, 0, 1);
    const yearEnd = new Date(targetYear, 11, 31);

    const s = getWeekStart(yearStart);

    let e = new Date(yearEnd);
    const endDay = e.getDay();
    if (endDay !== 6) {
      e.setDate(e.getDate() + (6 - endDay));
    }
    e.setHours(23, 59, 59, 999);

    return { start: s, end: e, yearStart, yearEnd };
  }, [year]);

  // 计算颜色等级
  const getLevel = (count: number, maxCount: number): number => {
    if (count === 0) return 0;
    if (maxCount === 0) return 0;

    const percentage = count / maxCount;
    if (percentage >= 0.75) return 4;
    if (percentage >= 0.5) return 3;
    if (percentage >= 0.25) return 2;
    return 1;
  };

  // 生成所有天的数据并按周分组
  const { weeks } = useMemo(() => {
    const allDays: DayData[] = [];
    const currentDate = new Date(start);
    let max = 0;

    // 生成所有日期
    while (currentDate <= end) {
      const key = formatDate(currentDate);
      const inRange =
        currentDate.getTime() >= yearStart.getTime() &&
        currentDate.getTime() <= yearEnd.getTime();
      const count = inRange ? valueMap.get(key) || 0 : 0;
      if (count > max) max = count;

      allDays.push({
        date: new Date(currentDate),
        count,
        level: 0, // 稍后计算
        inRange,
      });

      currentDate.setDate(currentDate.getDate() + 1);
    }

    // 计算等级
    allDays.forEach((day) => {
      day.level = getLevel(day.count, max);
    });

    // 按周分组
    const weekGroups: DayData[][] = [];
    let currentWeek: DayData[] = [];

    allDays.forEach((day) => {
      if (day.date.getDay() === 0 && currentWeek.length > 0) {
        weekGroups.push(currentWeek);
        currentWeek = [];
      }
      currentWeek.push(day);
    });
    if (currentWeek.length > 0) {
      weekGroups.push(currentWeek);
    }

    return { weeks: weekGroups };
  }, [start, end, valueMap, yearStart, yearEnd]);

  // 计算月份标签位置
  const monthLabels = useMemo(() => {
    const labels: { text: string; x: number }[] = [];
    let lastMonth = -1;

    weeks.forEach((week, weekIndex) => {
      const firstInRange = week.find((d) => d.inRange);
      if (!firstInRange) return;

      const month = firstInRange.date.getMonth();
      if (month !== lastMonth) {
        const dayOfMonth = firstInRange.date.getDate();
        const prevWeek = weeks[weekIndex - 1];
        const prevFirstInRange = prevWeek?.find((d) => d.inRange);
        const prevMonth = prevFirstInRange?.date.getMonth();

        if (dayOfMonth <= 7 || prevMonth !== month) {
          labels.push({
            text: MONTH_LABELS[month],
            x: weekIndex * (cellSize + cellGap),
          });
        }
        lastMonth = month;
      }
    });

    return labels;
  }, [weeks, cellSize, cellGap, yearStart, yearEnd]);

  // 获取颜色 class（使用 Tailwind/DaisyUI 以适配主题）
  const getColorClass = (level: number): string => {
    if (level === 0) return emptyColor;
    return colors[level - 1] || colors[colors.length - 1];
  };

  // 计算 SVG 尺寸
  const leftPadding = showWeekdayLabels ? 30 : 0;
  const topPadding = showMonthLabels ? 20 : 0;
  const svgWidth = leftPadding + weeks.length * (cellSize + cellGap) - cellGap;
  const svgHeight = topPadding + 7 * (cellSize + cellGap) - cellGap;

  return (
    <div className={`github-heatmap ${className}`}>
      <svg
        width={svgWidth}
        height={svgHeight}
        className="github-heatmap-svg"
        style={{
          fontFamily:
            "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
        }}
      >
        {/* 月份标签 */}
        {showMonthLabels && (
          <g className="months" transform={`translate(${leftPadding}, 0)`}>
            {monthLabels.map((label, i) => (
              <text
                key={i}
                x={label.x}
                y={12}
                fontSize={10}
                fill="currentColor"
                opacity={0.6}
                style={{ userSelect: "none" }}
              >
                {label.text}
              </text>
            ))}
          </g>
        )}

        {/* 星期标签 */}
        {showWeekdayLabels && (
          <g className="weekdays" transform={`translate(0, ${topPadding})`}>
            {WEEKDAY_LABELS.map((label, day) => {
              if (!label) return null;
              return (
                <text
                  key={day}
                  x={0}
                  y={day * (cellSize + cellGap) + cellSize / 2 + 3}
                  fontSize={9}
                  fill="currentColor"
                  opacity={0.6}
                  textAnchor="start"
                  style={{ userSelect: "none" }}
                >
                  {label}
                </text>
              );
            })}
          </g>
        )}

        {/* 贡献方块 */}
        <g
          className="contributions"
          transform={`translate(${leftPadding}, ${topPadding})`}
        >
          {weeks.map((week, weekIndex) => (
            <g
              key={weekIndex}
              transform={`translate(${weekIndex * (cellSize + cellGap)}, 0)`}
            >
              {week.map((day) => {
                if (!day.inRange) return null;
                const y = day.date.getDay() * (cellSize + cellGap);
                const colorClass = getColorClass(day.level);

                return (
                  <rect
                    key={formatDate(day.date)}
                    x={0}
                    y={y}
                    width={cellSize}
                    height={cellSize}
                    rx={2}
                    ry={2}
                    stroke="rgba(27, 31, 35, 0.06)"
                    strokeWidth={1}
                    className={`contribution-day ${colorClass}`}
                    data-date={formatDate(day.date)}
                    data-count={day.count}
                    data-level={day.level}
                    style={{ cursor: "pointer" }}
                  >
                    <title>
                      {formatDate(day.date)}: {day.count}{" "}
                      {day.count === 1 ? "photo" : "photos"}
                    </title>
                  </rect>
                );
              })}
            </g>
          ))}
        </g>
      </svg>

      {/* 图例 */}
      <div
        className="github-heatmap-legend"
        style={{
          display: "flex",
          alignItems: "center",
          gap: "4px",
          marginTop: "8px",
          fontSize: "11px",
          color: "currentColor",
          opacity: 0.6,
        }}
      >
        <span>Less</span>
        <div style={{ display: "flex", gap: "3px" }}>
          <div
            style={{
              width: cellSize,
              height: cellSize,
              borderRadius: "2px",
            }}
            className={`border border-base-300 ${emptyColor}`}
          />
          {colors.map((color, i) => (
            <div
              key={i}
              style={{
                width: cellSize,
                height: cellSize,
                borderRadius: "2px",
              }}
              className={`border border-base-300 ${color}`}
            />
          ))}
        </div>
        <span>More</span>
      </div>
    </div>
  );
};

export default GitHubStyleHeatmap;

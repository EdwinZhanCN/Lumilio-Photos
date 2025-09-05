import React from "react";
import { CameraIcon, ClockIcon } from "@heroicons/react/24/outline";
import { CalendarHeatmap } from "@/components/Heatmap";
import "@/styles/heatmap.css";

export type HeatmapValue = {
  date: string | Date;
  count: number;
};

export type FocalStat = {
  label: string;
  value: number; // percentage 0-100
};

export type ComboStat = {
  combo: string;
  rate: number; // percentage 0-100
};

export type StatsCardsProps = {
  className?: string;
  focalStats?: FocalStat[];
  timeDistribution?: number; // percentage 0-100
  combos?: ComboStat[];
  heatmapValues?: HeatmapValue[];
};

const DEFAULT_FOCAL_STATS: FocalStat[] = [
  { label: "24mm", value: 35 },
];

const DEFAULT_COMBOS: ComboStat[] = [
  { combo: "Canon EOS R5 + RF24-70mm", rate: 45 },
  { combo: "Sony A7IV + FE 24-70mm GM", rate: 30 },
  { combo: "Fujifilm X-T4 + XF16-55mm", rate: 15 },
  { combo: "Nikon Z7II + Z 24-70mm", rate: 8 },
  { combo: "Leica Q3 + Summilux 28mm", rate: 2 },
];

function generateSampleHeatmapData(): HeatmapValue[] {
  const startDate = new Date("2024-01-01");
  const endDate = new Date("2024-12-31");
  const data: HeatmapValue[] = [];
  const currentDate = new Date(startDate);

  while (currentDate <= endDate) {
    const count = Math.floor(Math.random() * 5);
    if (Math.random() > 0.7) {
      data.push({
        date: currentDate.toISOString().split("T")[0],
        count,
      });
    }
    currentDate.setDate(currentDate.getDate() + 1);
  }
  return data;
}

const StatsCards: React.FC<StatsCardsProps> = ({
  className = "",
  focalStats = DEFAULT_FOCAL_STATS,
  timeDistribution = 70,
  combos = DEFAULT_COMBOS,
  heatmapValues,
}) => {
  const values = React.useMemo(
    () => heatmapValues ?? generateSampleHeatmapData(),
    [heatmapValues]
  );

  return (
    <section
      className={`grid grid-cols-1 md:grid-cols-2 gap-4 p-4 bg-base-200 rounded-3xl ${className}`}
    >
      {/* 常用焦段分布 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <CameraIcon className="size-5" />
            <h3 className="font-bold">常用焦段分布</h3>
          </div>
          <div className="text-sm space-y-3 mt-2">
            {focalStats.map((item) => (
              <div key={item.label} className="space-y-1">
                <div className="flex justify-between">
                  <span>{item.label}</span>
                  <span className="text-primary">{item.value}%</span>
                </div>
                <progress
                  className="progress progress-primary w-full"
                  value={item.value}
                  max={100}
                />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 拍摄时段分布 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <ClockIcon className="size-5" />
            <h3 className="font-bold">拍摄时段分布</h3>
          </div>
          <div
            className="radial-progress text-primary mt-2"
            style={{ ["--value" as any]: timeDistribution }}
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={timeDistribution}
          >
            {timeDistribution}%
          </div>
          <p className="text-sm mt-2">黄金时段拍摄占比</p>
        </div>
      </div>

      {/* 常用相机镜头组合 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <CameraIcon className="size-5" />
            <h3 className="font-bold">常用相机镜头组合</h3>
          </div>
          <div className="text-sm space-y-2 mt-2">
            {combos.map((item, i) => (
              <div key={`${item.combo}-${i}`} className="space-y-1">
                <div className="flex justify-between text-xs">
                  <span>{item.combo}</span>
                  <span className="text-primary">{item.rate}%</span>
                </div>
                <progress
                  className="progress progress-primary w-full h-1"
                  value={item.rate}
                  max={100}
                />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 拍摄活跃热力图 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <ClockIcon className="size-5" />
            <h3 className="font-bold">拍摄活跃热力图</h3>
          </div>
          <div className="mt-2">
            <CalendarHeatmap
              values={values}
              classForValue={(value) => {
                if (!value) return "color-empty";
                const c = Math.max(0, Math.min(5, value.count));
                return `color-scale-${c}`;
              }}
              tooltipDataAttrs={(value) => {
                if (!value) return {};
                return {
                  "data-tip": `${value.date} 拍摄了 ${value.count} 张`,
                };
              }}
            />
          </div>
        </div>
      </div>
    </section>
  );
};

export default StatsCards;

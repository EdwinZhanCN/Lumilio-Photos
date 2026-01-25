import React, { useMemo, useState, useEffect } from "react";
import { CameraIcon, ClockIcon } from "@heroicons/react/24/outline";
import { GitHubStyleHeatmap } from "@/components/Heatmap";
import { usePhotoStats } from "../hooks/usePhotoStats";

export type StatsCardsProps = {
  className?: string;
};

const StatsCards: React.FC<StatsCardsProps> = ({ className = "" }) => {
  const {
    focalLengthData,
    cameraLensData,
    timeDistributionData,
    heatmapData,
    availableYears,
    heatmapLoading,
    isLoading,
    error,
    refetchHeatmap,
  } = usePhotoStats({
    autoFetch: true,
    cameraLensLimit: 5,
    timeDistributionType: "hourly",
  });

  const [selectedYear, setSelectedYear] = useState<number | null>(null);

  // 设置默认年份为最新年份
  useEffect(() => {
    if (availableYears.length > 0 && selectedYear === null) {
      setSelectedYear(availableYears[0]);
    }
  }, [availableYears, selectedYear]);

  // 当选择年份改变时重新获取热力图数据
  const handleYearChange = (year: number) => {
    setSelectedYear(year);
    refetchHeatmap(year);
  };

  // Transform focal length data to percentage format
  const focalStats = useMemo(() => {
    const total = focalLengthData?.total ?? 0;
    if (!focalLengthData || total === 0) return [];

    return (focalLengthData.data ?? []).slice(0, 5).map((item) => ({
      label: `${item.focal_length ?? 0}mm`,
      value: Math.round(((item.count ?? 0) / total) * 100),
    }));
  }, [focalLengthData]);

  // Transform camera lens data to percentage format
  const combos = useMemo(() => {
    const total = cameraLensData?.total ?? 0;
    if (!cameraLensData || total === 0) return [];

    return (cameraLensData.data ?? []).map((item) => ({
      combo: `${item.camera_model ?? 'Unknown'} + ${item.lens_model ?? 'Unknown'}`,
      rate: Math.round(((item.count ?? 0) / total) * 100),
    }));
  }, [cameraLensData]);

  // Calculate multiple time-of-day percentages (golden / blue / sunrise / sunset)
  const { goldenPercent, bluePercent, sunrisePercent, sunsetPercent } =
    useMemo(() => {
      if (!timeDistributionData || timeDistributionData.type !== "hourly") {
        return {
          goldenPercent: 0,
          bluePercent: 0,
          sunrisePercent: 0,
          sunsetPercent: 0,
        };
      }

      const dataArray = timeDistributionData.data ?? [];
      const totalCount = dataArray.reduce(
        (sum, item) => sum + (item.count ?? 0),
        0,
      );
      if (totalCount === 0) {
        return {
          goldenPercent: 0,
          bluePercent: 0,
          sunrisePercent: 0,
          sunsetPercent: 0,
        };
      }

      const sumHours = (hours: number[]) =>
        dataArray
          .filter((item) => hours.includes(item.value ?? 0))
          .reduce((sum, item) => sum + (item.count ?? 0), 0);

      const goldenPercent = Math.round(
        (sumHours([5, 6, 7, 8, 17, 18, 19, 20]) / totalCount) * 100,
      );
      const bluePercent = Math.round(
        (sumHours([4, 5, 20, 21]) / totalCount) * 100,
      );
      const sunrisePercent = Math.round(
        (sumHours([5, 6, 7]) / totalCount) * 100,
      );
      const sunsetPercent = Math.round(
        (sumHours([17, 18, 19]) / totalCount) * 100,
      );

      return { goldenPercent, bluePercent, sunrisePercent, sunsetPercent };
    }, [timeDistributionData]);

  // Transform heatmap data for GitHubStyleHeatmap
  const heatmapValues = useMemo(() => {
    if (!heatmapData || !heatmapData.data) return [];
    return heatmapData.data
      .filter((item) => item.date !== undefined)
      .map((item) => ({
        date: item.date as string,
        count: item.count ?? 0,
      }));
  }, [heatmapData]);

  // Show loading state
  if (isLoading) {
    return (
      <section
        className={`grid grid-cols-1 md:grid-cols-2 gap-4 p-4 bg-base-200 rounded-3xl ${className}`}
      >
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="card bg-base-100 shadow-sm">
            <div className="card-body">
              <div className="flex items-center gap-2">
                <div className="skeleton h-5 w-5"></div>
                <div className="skeleton h-4 w-32"></div>
              </div>
              <div className="skeleton h-32 w-full mt-4"></div>
            </div>
          </div>
        ))}
      </section>
    );
  }

  // Show error state
  if (error) {
    return (
      <section
        className={`grid grid-cols-1 md:grid-cols-2 gap-4 p-4 bg-base-200 rounded-3xl ${className}`}
      >
        <div className="col-span-full card bg-base-100 shadow-sm">
          <div className="card-body">
            <div className="alert alert-error">
              <span>加载统计数据失败: {error}</span>
            </div>
          </div>
        </div>
      </section>
    );
  }

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
          {focalStats.length > 0 ? (
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
          ) : (
            <div className="flex items-center justify-center h-32 text-base-content/50">
              <p className="text-sm">暂无焦距数据</p>
            </div>
          )}
        </div>
      </div>

      {/* 拍摄时段分布 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <ClockIcon className="size-5" />
            <h3 className="font-bold">拍摄时段分布</h3>
          </div>
          <div className="grid grid-cols-2 gap-4 items-center justify-center mt-2">
            <div className="flex flex-col items-center gap-1">
              <div
                className="radial-progress text-primary"
                style={{ ["--value" as any]: goldenPercent }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={goldenPercent}
              >
                {goldenPercent}%
              </div>
              <p className="text-xs text-primary">黄金</p>
            </div>

            <div className="flex flex-col items-center gap-1">
              <div
                className="radial-progress text-info"
                style={{ ["--value" as any]: bluePercent }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={bluePercent}
              >
                {bluePercent}%
              </div>
              <p className="text-xs text-info">蓝调</p>
            </div>

            <div className="flex flex-col items-center gap-1">
              <div
                className="radial-progress text-warning"
                style={{ ["--value" as any]: sunrisePercent }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={sunrisePercent}
              >
                {sunrisePercent}%
              </div>
              <p className="text-xs text-warning">日出</p>
            </div>

            <div className="flex flex-col items-center gap-1">
              <div
                className="radial-progress text-secondary"
                style={{ ["--value" as any]: sunsetPercent }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={sunsetPercent}
              >
                {sunsetPercent}%
              </div>
              <p className="text-xs text-secondary">日落</p>
            </div>
          </div>
        </div>
      </div>

      {/* 常用相机镜头组合 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center gap-2 text-primary">
            <CameraIcon className="size-5" />
            <h3 className="font-bold">常用相机镜头组合</h3>
          </div>
          {combos.length > 0 ? (
            <div className="text-sm space-y-2 mt-2">
              {combos.map((item, i) => (
                <div key={`${item.combo}-${i}`} className="space-y-1">
                  <div className="flex justify-between text-xs">
                    <span className="truncate" title={item.combo}>
                      {item.combo}
                    </span>
                    <span className="text-primary ml-2 flex-shrink-0">
                      {item.rate}%
                    </span>
                  </div>
                  <progress
                    className="progress progress-primary w-full h-1"
                    value={item.rate}
                    max={100}
                  />
                </div>
              ))}
            </div>
          ) : (
            <div className="flex items-center justify-center h-32 text-base-content/50">
              <p className="text-sm">暂无相机镜头数据</p>
            </div>
          )}
        </div>
      </div>

      {/* 拍摄活跃热力图 */}
      <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
        <div className="card-body">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2 text-primary">
              <ClockIcon className="size-5" />
              <h3 className="font-bold">拍摄活跃热力图</h3>
            </div>
            {availableYears.length > 0 && (
              <div className="flex items-center gap-2">
                <div className="flex gap-1 overflow-x-auto max-w-xs">
                  {availableYears.map((year) => (
                    <button
                      key={year}
                      onClick={() => handleYearChange(year)}
                      disabled={heatmapLoading}
                      className={`btn btn-xs ${selectedYear === year ? "btn-primary" : "btn-ghost"
                        } ${heatmapLoading ? "btn-disabled" : ""}`}
                    >
                      {heatmapLoading && selectedYear === year ? (
                        <span className="loading loading-spinner loading-xs" />
                      ) : (
                        year
                      )}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
          {heatmapLoading ? (
            <div className="flex items-center justify-center h-32 text-base-content/70">
              <span className="loading loading-spinner loading-md text-primary" />
            </div>
          ) : heatmapValues.length > 0 && selectedYear ? (
            <div className="overflow-x-auto pb-2">
              <GitHubStyleHeatmap
                values={heatmapValues}
                year={selectedYear}
                showMonthLabels={true}
                showWeekdayLabels={true}
                cellSize={11}
                cellGap={3}
              />
            </div>
          ) : (
            <div className="flex items-center justify-center h-32 text-base-content/50">
              <p className="text-sm">暂无活跃度数据</p>
            </div>
          )}
        </div>
      </div>
    </section>
  );
};

export default StatsCards;

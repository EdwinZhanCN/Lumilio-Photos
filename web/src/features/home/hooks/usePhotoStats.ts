import { useState, useEffect, useCallback } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type FocalLengthDistributionResponse =
  Schemas["handler.FocalLengthDistributionResponse"];
type CameraLensStatsResponse = Schemas["handler.CameraLensStatsResponse"];
type TimeDistributionResponse = Schemas["handler.TimeDistributionResponse"];
type HeatmapResponse = Schemas["handler.HeatmapResponse"];
type AvailableYearsResponse = Schemas["handler.AvailableYearsResponse"];

type TimeDistributionType = "hourly" | "monthly";

interface UsePhotoStatsOptions {
  autoFetch?: boolean;
  cameraLensLimit?: number;
  timeDistributionType?: TimeDistributionType;
}

interface UsePhotoStatsReturn {
  focalLengthData: FocalLengthDistributionResponse | null;
  cameraLensData: CameraLensStatsResponse | null;
  timeDistributionData: TimeDistributionResponse | null;
  heatmapData: HeatmapResponse | null;
  availableYears: number[];
  heatmapLoading: boolean;
  isLoading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
  refetchHeatmap: (year: number) => Promise<void>;
}

/**
 * Custom hook to fetch photo statistics
 * @param options - Configuration options
 * @returns Statistics data and loading states
 */
export function usePhotoStats(
  options: UsePhotoStatsOptions = {},
): UsePhotoStatsReturn {
  const {
    autoFetch = true,
    cameraLensLimit = 10,
    timeDistributionType = "hourly",
  } = options;

  const [focalLengthData, setFocalLengthData] =
    useState<FocalLengthDistributionResponse | null>(null);
  const [cameraLensData, setCameraLensData] =
    useState<CameraLensStatsResponse | null>(null);
  const [timeDistributionData, setTimeDistributionData] =
    useState<TimeDistributionResponse | null>(null);
  const [heatmapData, setHeatmapData] = useState<HeatmapResponse | null>(null);
  const [availableYears, setAvailableYears] = useState<number[]>([]);
  const [heatmapLoading, setHeatmapLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { mutateAsync: fetchFocalLength } = $api.useMutation(
    "get",
    "/api/v1/stats/focal-length",
  );
  const { mutateAsync: fetchCameraLensStats } = $api.useMutation(
    "get",
    "/api/v1/stats/camera-lens",
  );
  const { mutateAsync: fetchTimeDistribution } = $api.useMutation(
    "get",
    "/api/v1/stats/time-distribution",
  );
  const { mutateAsync: fetchAvailableYears } = $api.useMutation(
    "get",
    "/api/v1/stats/available-years",
  );
  const { mutateAsync: fetchDailyActivity } = $api.useMutation(
    "get",
    "/api/v1/stats/daily-activity",
  );

  const fetchAllStats = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const [focalResponse, cameraLensResponse, timeResponse, yearsResponse] =
        await Promise.all([
          fetchFocalLength({}),
          fetchCameraLensStats({
            params: { query: { limit: cameraLensLimit } },
          }),
          fetchTimeDistribution({
            params: { query: { type: timeDistributionType } },
          }),
          fetchAvailableYears({}),
        ]);

      // Check focal length response (openapi-fetch format)
      const focalData = focalResponse as ApiResult<FocalLengthDistributionResponse> | undefined;
      if (focalData?.code === 0 && focalData.data) {
        setFocalLengthData(focalData.data);
      }

      // Check camera lens response
      const cameraLensData =
        cameraLensResponse as ApiResult<CameraLensStatsResponse> | undefined;
      if (cameraLensData?.code === 0 && cameraLensData.data) {
        setCameraLensData(cameraLensData.data);
      }

      // Check time distribution response
      const timeDistributionData =
        timeResponse as ApiResult<TimeDistributionResponse> | undefined;
      if (timeDistributionData?.code === 0 && timeDistributionData.data) {
        setTimeDistributionData(timeDistributionData.data);
      }

      // Check years response
      const yearsData =
        yearsResponse as ApiResult<AvailableYearsResponse> | undefined;
      if (yearsData?.code === 0 && yearsData.data) {
        const years = yearsData.data.years ?? [];
        setAvailableYears(years);

        // Fetch heatmap for the most recent year
        setHeatmapLoading(true);
        try {
          if (years.length > 0) {
            const latestYear = years[0];
            const startDate = new Date(latestYear, 0, 1);
            const endDate = new Date(latestYear, 11, 31);
            const daysDiff = Math.ceil(
              (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24),
            );

            const heatmapResponse = await fetchDailyActivity({
              params: { query: { days: daysDiff + 365 } },
            });
            const heatmapData =
              heatmapResponse as ApiResult<HeatmapResponse> | undefined;
            if (heatmapData?.code === 0 && heatmapData.data) {
              setHeatmapData(heatmapData.data);
            }
          }
        } finally {
          setHeatmapLoading(false);
        }
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch statistics";
      setError(errorMessage);
      console.error("Error fetching photo stats:", err);
    } finally {
      setIsLoading(false);
    }
  }, [
    cameraLensLimit,
    timeDistributionType,
    fetchAvailableYears,
    fetchCameraLensStats,
    fetchDailyActivity,
    fetchFocalLength,
    fetchTimeDistribution,
  ]);

  const refetchHeatmap = useCallback(async (year: number) => {
    setHeatmapLoading(true);
    setError(null);

    try {
      const startDate = new Date(year, 0, 1);
      const endDate = new Date(year, 11, 31);
      const daysDiff = Math.ceil(
        (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24),
      );

      const heatmapResponse = await fetchDailyActivity({
        params: { query: { days: daysDiff + 365 } },
      });
      const heatmapData =
        heatmapResponse as ApiResult<HeatmapResponse> | undefined;

      if (heatmapData?.code === 0 && heatmapData.data) {
        setHeatmapData(heatmapData.data);
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch heatmap";
      setError(errorMessage);
      console.error("Error fetching heatmap:", err);
    } finally {
      setHeatmapLoading(false);
    }
  }, [fetchDailyActivity]);

  useEffect(() => {
    if (autoFetch) {
      fetchAllStats();
    }
  }, [autoFetch, fetchAllStats]);

  return {
    focalLengthData,
    cameraLensData,
    timeDistributionData,
    heatmapData,
    availableYears,
    heatmapLoading,
    isLoading,
    error,
    refetch: fetchAllStats,
    refetchHeatmap,
  };
}

import { useState, useEffect, useCallback, useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

type FocalLengthDistributionResponse = Schemas["handler.FocalLengthDistributionResponse"];
type CameraLensStatsResponse = Schemas["handler.CameraLensStatsResponse"];
type TimeDistributionResponse = Schemas["handler.TimeDistributionResponse"];
type HeatmapResponse = Schemas["handler.HeatmapResponse"];
type AvailableYearsResponse = Schemas["handler.AvailableYearsResponse"];

type TimeDistributionType = "hourly" | "monthly";

interface UsePhotoStatsOptions {
  autoFetch?: boolean;
  cameraLensLimit?: number;
  timeDistributionType?: TimeDistributionType;
  repositoryId?: string;
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
export function usePhotoStats(options: UsePhotoStatsOptions = {}): UsePhotoStatsReturn {
  const {
    autoFetch = true,
    cameraLensLimit = 10,
    timeDistributionType = "hourly",
    repositoryId,
  } = options;

  const [focalLengthData, setFocalLengthData] = useState<FocalLengthDistributionResponse | null>(
    null,
  );
  const [cameraLensData, setCameraLensData] = useState<CameraLensStatsResponse | null>(null);
  const [timeDistributionData, setTimeDistributionData] = useState<TimeDistributionResponse | null>(
    null,
  );
  const [heatmapData, setHeatmapData] = useState<HeatmapResponse | null>(null);
  const [availableYears, setAvailableYears] = useState<number[]>([]);
  const [heatmapLoading, setHeatmapLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { mutateAsync: fetchFocalLength } = $api.useMutation("get", "/api/v1/stats/focal-length");
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

  const repositoryScope = useMemo(
    () => (repositoryId ? { repository_id: repositoryId } : undefined),
    [repositoryId],
  );

  const fetchAllStats = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const [focalResponse, cameraLensResponse, timeResponse, yearsResponse] = await Promise.all([
        fetchFocalLength({
          params: { query: repositoryScope },
        }),
        fetchCameraLensStats({
          params: {
            query: {
              limit: cameraLensLimit,
              ...repositoryScope,
            },
          },
        }),
        fetchTimeDistribution({
          params: {
            query: {
              type: timeDistributionType,
              ...repositoryScope,
            },
          },
        }),
        fetchAvailableYears({
          params: { query: repositoryScope },
        }),
      ]);

      const focalData = focalResponse as FocalLengthDistributionResponse | undefined;
      if (focalData) {
        setFocalLengthData(focalData);
      }

      const cameraLensData = cameraLensResponse as CameraLensStatsResponse | undefined;
      if (cameraLensData) {
        setCameraLensData(cameraLensData);
      }

      const timeDistributionData = timeResponse as TimeDistributionResponse | undefined;
      if (timeDistributionData) {
        setTimeDistributionData(timeDistributionData);
      }

      const yearsData = yearsResponse as AvailableYearsResponse | undefined;
      if (yearsData) {
        const years = yearsData.years ?? [];
        setAvailableYears(years);

        // Fetch heatmap for the most recent year
        setHeatmapLoading(true);
        try {
          if (years.length > 0) {
            const latestYear = years[0];

            const heatmapResponse = await fetchDailyActivity({
              params: {
                query: {
                  year: latestYear,
                  ...repositoryScope,
                },
              },
            });
            const heatmapData = heatmapResponse as HeatmapResponse | undefined;
            if (heatmapData) {
              setHeatmapData(heatmapData);
            }
          }
        } finally {
          setHeatmapLoading(false);
        }
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Failed to fetch statistics";
      setError(errorMessage);
      console.error("Error fetching photo stats:", err);
    } finally {
      setIsLoading(false);
    }
  }, [
    cameraLensLimit,
    timeDistributionType,
    repositoryScope,
    fetchAvailableYears,
    fetchCameraLensStats,
    fetchDailyActivity,
    fetchFocalLength,
    fetchTimeDistribution,
  ]);

  const refetchHeatmap = useCallback(
    async (year: number) => {
      setHeatmapLoading(true);
      setError(null);

      try {
        const heatmapResponse = await fetchDailyActivity({
          params: {
            query: {
              year,
              ...repositoryScope,
            },
          },
        });
        const heatmapData = heatmapResponse as HeatmapResponse | undefined;

        if (heatmapData) {
          setHeatmapData(heatmapData);
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Failed to fetch heatmap";
        setError(errorMessage);
        console.error("Error fetching heatmap:", err);
      } finally {
        setHeatmapLoading(false);
      }
    },
    [fetchDailyActivity, repositoryScope],
  );

  useEffect(() => {
    if (autoFetch) {
      void fetchAllStats();
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

import { useState, useEffect, useCallback } from "react";
import { statsService } from "@/services";
import type {
  FocalLengthDistributionResponse,
  CameraLensStatsResponse,
  TimeDistributionResponse,
  HeatmapResponse,
} from "@/services";

interface UsePhotoStatsOptions {
  autoFetch?: boolean;
  cameraLensLimit?: number;
  timeDistributionType?: "hourly" | "monthly";
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

  const fetchAllStats = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const [focalResponse, cameraLensResponse, timeResponse, yearsResponse] =
        await Promise.all([
          statsService.getFocalLengthDistribution(),
          statsService.getCameraLensStats(cameraLensLimit),
          statsService.getTimeDistribution(timeDistributionType),
          statsService.getAvailableYears(),
        ]);

      // Check focal length response
      if (focalResponse.data.code === 0 && focalResponse.data.data) {
        setFocalLengthData(focalResponse.data.data);
      }

      // Check camera lens response
      if (cameraLensResponse.data.code === 0 && cameraLensResponse.data.data) {
        setCameraLensData(cameraLensResponse.data.data);
      }

      // Check time distribution response
      if (timeResponse.data.code === 0 && timeResponse.data.data) {
        setTimeDistributionData(timeResponse.data.data);
      }

      // Check years response
      if (yearsResponse.data.code === 0 && yearsResponse.data.data) {
        const years = yearsResponse.data.data.years;
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

            const heatmapResponse = await statsService.getDailyActivityHeatmap(
              daysDiff + 365,
            );
            if (heatmapResponse.data.code === 0 && heatmapResponse.data.data) {
              setHeatmapData(heatmapResponse.data.data);
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
  }, [cameraLensLimit, timeDistributionType]);

  const refetchHeatmap = useCallback(async (year: number) => {
    setHeatmapLoading(true);
    setError(null);

    try {
      const startDate = new Date(year, 0, 1);
      const endDate = new Date(year, 11, 31);
      const daysDiff = Math.ceil(
        (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24),
      );

      const heatmapResponse = await statsService.getDailyActivityHeatmap(
        daysDiff + 365,
      );

      if (heatmapResponse.data.code === 0 && heatmapResponse.data.data) {
        setHeatmapData(heatmapResponse.data.data);
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to fetch heatmap";
      setError(errorMessage);
      console.error("Error fetching heatmap:", err);
    } finally {
      setHeatmapLoading(false);
    }
  }, []);

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

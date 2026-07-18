import { useCallback, useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

type FocalLengthDistributionResponse = Schemas["handler.FocalLengthDistributionResponse"];
type CameraLensStatsResponse = Schemas["handler.CameraLensStatsResponse"];
type TimeDistributionResponse = Schemas["handler.TimeDistributionResponse"];
type HeatmapResponse = Schemas["handler.HeatmapResponse"];

type TimeDistributionType = "hourly" | "monthly";

interface UsePhotoStatsOptions {
  autoFetch?: boolean;
  cameraLensLimit?: number;
  timeDistributionType?: TimeDistributionType;
  repositoryId?: string;
  heatmapYear?: number | null;
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
    heatmapYear,
  } = options;

  const repositoryScope = useMemo(
    () => (repositoryId ? { repository_id: repositoryId } : undefined),
    [repositoryId],
  );

  const commonOptions = {
    enabled: autoFetch,
    staleTime: 2 * 60 * 1000,
    gcTime: 5 * 60 * 1000,
  } as const;
  const focalQuery = $api.useQuery(
    "get",
    "/api/v1/stats/focal-length",
    { params: { query: repositoryScope } },
    commonOptions,
  );
  const cameraQuery = $api.useQuery(
    "get",
    "/api/v1/stats/camera-lens",
    { params: { query: { limit: cameraLensLimit, ...repositoryScope } } },
    commonOptions,
  );
  const timeQuery = $api.useQuery(
    "get",
    "/api/v1/stats/time-distribution",
    { params: { query: { type: timeDistributionType, ...repositoryScope } } },
    commonOptions,
  );
  const yearsQuery = $api.useQuery(
    "get",
    "/api/v1/stats/available-years",
    { params: { query: repositoryScope } },
    commonOptions,
  );
  const heatmapQuery = $api.useQuery(
    "get",
    "/api/v1/stats/daily-activity",
    {
      params: {
        query: {
          year: heatmapYear ?? new Date().getFullYear(),
          ...repositoryScope,
        },
      },
    },
    { ...commonOptions, enabled: autoFetch && heatmapYear !== null && heatmapYear !== undefined },
  );
  const queries = [focalQuery, cameraQuery, timeQuery, yearsQuery, heatmapQuery];
  const firstError = queries.find((query) => query.error)?.error;
  const refetch = useCallback(async () => {
    await Promise.all(queries.map((query) => query.refetch()));
  }, [focalQuery, cameraQuery, timeQuery, yearsQuery, heatmapQuery]);

  return {
    focalLengthData: focalQuery.data ?? null,
    cameraLensData: cameraQuery.data ?? null,
    timeDistributionData: timeQuery.data ?? null,
    heatmapData: heatmapQuery.data ?? null,
    availableYears: yearsQuery.data?.years ?? [],
    heatmapLoading: heatmapQuery.isFetching,
    isLoading: queries.some((query) => query.isLoading),
    error:
      firstError instanceof Error
        ? firstError.message
        : firstError
          ? "Failed to fetch statistics"
          : null,
    refetch,
  };
}

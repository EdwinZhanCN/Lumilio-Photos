import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

type AssetIndexingStatsResponseDTO = Schemas["dto.AssetIndexingStatsResponseDTO"];
export type RebuildAssetIndexesPayload = Schemas["dto.RebuildAssetIndexesRequestDTO"];
export type RebuildAssetIndexesResponse = Schemas["dto.RebuildAssetIndexesResponseDTO"];

type AssetIndexingTaskStatsDTO = Schemas["dto.AssetIndexingTaskStatsDTO"];

type AssetIndexingTaskStats = {
  indexedCount: number;
  queuedJobs: number;
  totalCount: number;
  coverage: number;
};

export type AssetIndexingStats = {
  photoTotal: number;
  videoTotal: number;
  reindexJobs: number;
  tasks: {
    semantic: AssetIndexingTaskStats;
    video_semantic: AssetIndexingTaskStats;
    bioclip: AssetIndexingTaskStats;
    ocr: AssetIndexingTaskStats;
    face: AssetIndexingTaskStats;
  };
};

function normalizeTaskStats(
  task: AssetIndexingTaskStatsDTO | undefined,
  fallbackTotal: number,
): AssetIndexingTaskStats {
  const indexedCount = task?.indexed_count ?? 0;
  const queuedJobs = task?.queued_jobs ?? 0;
  const totalCount = task?.total_count ?? fallbackTotal;

  return {
    indexedCount,
    queuedJobs,
    totalCount,
    coverage: totalCount > 0 ? indexedCount / totalCount : 0,
  };
}

function normalizeAssetIndexingStats(
  data?: AssetIndexingStatsResponseDTO,
): AssetIndexingStats | undefined {
  if (!data) {
    return undefined;
  }

  const photoTotal = data.photo_total ?? 0;
  const videoTotal = data.video_total ?? 0;

  return {
    photoTotal,
    videoTotal,
    reindexJobs: data.reindex_jobs ?? 0,
    tasks: {
      semantic: normalizeTaskStats(data.tasks?.semantic, photoTotal),
      video_semantic: normalizeTaskStats(data.tasks?.video_semantic, videoTotal),
      bioclip: normalizeTaskStats(data.tasks?.bioclip, photoTotal),
      ocr: normalizeTaskStats(data.tasks?.ocr, photoTotal),
      face: normalizeTaskStats(data.tasks?.face, photoTotal),
    },
  };
}

export function useAssetIndexingStats(repositoryId?: string): UseQueryResult<
  AssetIndexingStatsResponseDTO,
  unknown
> & {
  stats?: AssetIndexingStats;
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/assets/indexing/stats",
    {
      params: {
        query: repositoryId ? { repository_id: repositoryId } : {},
      },
    },
    {
      refetchInterval: 15_000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<AssetIndexingStatsResponseDTO, unknown>;

  return {
    ...query,
    stats: normalizeAssetIndexingStats(query.data),
  };
}

export function useRebuildAssetIndexes() {
  const queryClient = useQueryClient();

  return $api.useMutation("post", "/api/v1/assets/indexing/rebuild", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/stats"],
      });
    },
  });
}

export function extractRebuildResponseData(raw: unknown): RebuildAssetIndexesResponse | undefined {
  if (raw && typeof raw === "object") {
    return raw as RebuildAssetIndexesResponse;
  }
  return undefined;
}

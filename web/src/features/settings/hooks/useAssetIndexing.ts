import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type IndexingRepositoryListResponseDTO =
  Schemas["dto.IndexingRepositoryListResponseDTO"];
type AssetIndexingStatsResponseDTO =
  Schemas["dto.AssetIndexingStatsResponseDTO"];
export type RebuildAssetIndexesPayload =
  Schemas["dto.RebuildAssetIndexesRequestDTO"];

type AssetIndexingTaskStatsDTO = Schemas["dto.AssetIndexingTaskStatsDTO"];

type AssetIndexingTaskStats = {
  indexedCount: number;
  queuedJobs: number;
  coverage: number;
};

export type AssetIndexingStats = {
  photoTotal: number;
  reindexJobs: number;
  tasks: {
    clip: AssetIndexingTaskStats;
    ocr: AssetIndexingTaskStats;
    caption: AssetIndexingTaskStats;
    face: AssetIndexingTaskStats;
  };
};

export type IndexingRepositoryOption = {
  id: string;
  name: string;
  path: string;
  isPrimary: boolean;
};

function normalizeIndexingRepositories(
  data?: IndexingRepositoryListResponseDTO,
): IndexingRepositoryOption[] {
  return (data?.repositories ?? []).map((repository) => ({
    id: repository.id ?? "",
    name: repository.name ?? "",
    path: repository.path ?? "",
    isPrimary: Boolean(repository.is_primary),
  }));
}

function normalizeTaskStats(
  task: AssetIndexingTaskStatsDTO | undefined,
  photoTotal: number,
): AssetIndexingTaskStats {
  const indexedCount = task?.indexed_count ?? 0;
  const queuedJobs = task?.queued_jobs ?? 0;

  return {
    indexedCount,
    queuedJobs,
    coverage: photoTotal > 0 ? indexedCount / photoTotal : 0,
  };
}

function normalizeAssetIndexingStats(
  data?: AssetIndexingStatsResponseDTO,
): AssetIndexingStats | undefined {
  if (!data) {
    return undefined;
  }

  const photoTotal = data.photo_total ?? 0;

  return {
    photoTotal,
    reindexJobs: data.reindex_jobs ?? 0,
    tasks: {
      clip: normalizeTaskStats(data.tasks?.clip, photoTotal),
      ocr: normalizeTaskStats(data.tasks?.ocr, photoTotal),
      caption: normalizeTaskStats(data.tasks?.caption, photoTotal),
      face: normalizeTaskStats(data.tasks?.face, photoTotal),
    },
  };
}

export function useIndexingRepositories(): UseQueryResult<
  ApiResult<IndexingRepositoryListResponseDTO>,
  unknown
> & {
  repositories: IndexingRepositoryOption[];
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/assets/indexing/repositories",
    {},
    {
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<IndexingRepositoryListResponseDTO>, unknown>;

  return {
    ...query,
    repositories: normalizeIndexingRepositories(query.data?.data),
  };
}

export function useAssetIndexingStats(repositoryId?: string): UseQueryResult<
  ApiResult<AssetIndexingStatsResponseDTO>,
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
  ) as UseQueryResult<ApiResult<AssetIndexingStatsResponseDTO>, unknown>;

  return {
    ...query,
    stats: normalizeAssetIndexingStats(query.data?.data),
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

import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { UseQueryResult } from "@tanstack/react-query";

type Schemas = components["schemas"];
type FeaturedAssetsResponse = Schemas["dto.FeaturedAssetsResponseDTO"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type UseFeaturedPhotosOptions = {
  count?: number;
  candidateLimit?: number;
  days?: number;
  seed?: string;
  repositoryId?: string;
};

export function useFeaturedPhotos(options: UseFeaturedPhotosOptions = {}) {
  const {
    count = 8,
    candidateLimit = 240,
    days = 3650,
    seed,
    repositoryId,
  } = options;

  const query = $api.useQuery(
    "get",
    "/api/v1/assets/featured",
    {
      params: {
        query: {
          count,
          candidate_limit: candidateLimit,
          days,
          seed,
          repository_id: repositoryId,
        },
      },
    },
    {
      staleTime: 5 * 60 * 1000,
      gcTime: 30 * 60 * 1000,
      retry: 1,
    },
  ) as UseQueryResult<ApiResult<FeaturedAssetsResponse>, unknown>;

  const payload = query.data?.data;

  return {
    assets: payload?.assets ?? [],
    count: payload?.count ?? 0,
    candidateCount: payload?.candidate_count ?? 0,
    seed: payload?.seed ?? "",
    strategy: payload?.strategy ?? "",
    generatedAt: payload?.generated_at_time,
    ...query,
  };
}

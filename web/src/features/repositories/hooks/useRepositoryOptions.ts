import type { UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";
import type { RepositoryOption } from "../repository.types";

type RepositoryListResponse = components["schemas"]["dto.IndexingRepositoryListResponseDTO"];

export function normalizeRepositoryOptions(data?: RepositoryListResponse): RepositoryOption[] {
  return (data?.repositories ?? []).map((repository) => ({
    id: repository.id ?? "",
    name: repository.name ?? "",
    path: repository.path ?? "",
    role: repository.role ?? "regular",
    isPrimary: repository.role === "primary" || Boolean(repository.is_primary),
  }));
}

export function useRepositoryOptions(): UseQueryResult<RepositoryListResponse, unknown> & {
  repositories: RepositoryOption[];
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/assets/indexing/repositories",
    {},
    {
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<RepositoryListResponse, unknown>;

  return {
    ...query,
    repositories: normalizeRepositoryOptions(query.data),
  };
}

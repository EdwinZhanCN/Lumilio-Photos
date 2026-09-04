import { $api } from "@/lib/http-commons/queryClient";
import { normalizeRepositoryOptions } from "../model/repositoryOptions";

export function useRepositoryOptions() {
  const query = $api.useQuery(
    "get",
    "/api/v1/assets/indexing/repositories",
    {},
    {
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
      select: normalizeRepositoryOptions,
    },
  );

  return {
    ...query,
    repositories: query.data ?? [],
  };
}

import { $api } from "@/lib/http-commons/queryClient";

export function useRepositoryRoots() {
  return $api.useQuery(
    "get",
    "/api/v1/repository-roots",
    {},
    {
      staleTime: 30_000,
      refetchOnWindowFocus: true,
    },
  );
}

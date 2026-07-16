import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export function useRepositoryCloudStatus(repositoryId: string, enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/repositories/{id}/cloud",
    {
      params: {
        path: {
          id: repositoryId,
        },
      },
    },
    {
      enabled: enabled && Boolean(repositoryId),
      // Only poll while an import is actively in progress; otherwise fetch once.
      // Avoids every repository card hammering this endpoint every 5s forever.
      refetchInterval: (query) => {
        const status = query.state.data?.latest_run?.status;
        return status === "running" || status === "queued" ? 5000 : false;
      },
      staleTime: 2000,
    },
  );
}

export function useStartRepositoryCloudImport() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/repositories/{id}/cloud/import", {
    onSuccess: (_data, variables) => {
      const id = variables?.params?.path?.id;
      if (id) {
        void queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/repositories/{id}/cloud"],
        });
      }
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/repositories"],
      });
    },
  });
}

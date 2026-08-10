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
        const sources = query.state.data?.sources ?? [];
        return sources.some(
          (source) =>
            source.latest_run?.status === "running" ||
            source.latest_run?.status === "queued" ||
            source.latest_run?.status === "cancelling",
        )
          ? 5000
          : false;
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

export function useBindRepositoryCloudSource() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/repositories/{id}/cloud/sources", {
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/repositories/{id}/cloud"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/repositories"],
      });
    },
  });
}

export function useCancelCloudImport() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/import-runs/{id}/cancel", {
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/repositories/{id}/cloud"],
      });
    },
  });
}

export function useResumeCloudImport() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/import-runs/{id}/resume", {
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/repositories/{id}/cloud"],
      });
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/repositories"],
      });
    },
  });
}

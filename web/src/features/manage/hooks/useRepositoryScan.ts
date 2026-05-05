import { useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

const invalidateRepositoryAwareQueries = async (
  queryClient: ReturnType<typeof useQueryClient>,
) => {
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: ["get", "/api/v1/assets/indexing/repositories"],
    }),
    queryClient.invalidateQueries({
      queryKey: ["post", "/api/v1/assets/list"],
    }),
    queryClient.invalidateQueries({
      queryKey: ["post", "/api/v1/assets/search"],
    }),
  ]);
};

export function useRepositoryScan() {
  const queryClient = useQueryClient();
  const scanMutation = $api.useMutation("post", "/api/v1/repositories/{id}/scan");
  const [scanningIds, setScanningIds] = useState<Set<string>>(
    () => new Set(),
  );

  const scanRepository = useCallback(
    async (repositoryId: string) => {
      setScanningIds((current) => new Set(current).add(repositoryId));
      try {
        await scanMutation.mutateAsync({
          params: {
            path: {
              id: repositoryId,
            },
          },
          body: {
            force: false,
          },
        });
        await invalidateRepositoryAwareQueries(queryClient);
      } finally {
        setScanningIds((current) => {
          const next = new Set(current);
          next.delete(repositoryId);
          return next;
        });
      }
    },
    [queryClient, scanMutation],
  );

  const scanRepositories = useCallback(
    async (repositoryIds: string[]) => {
      const uniqueIds = Array.from(new Set(repositoryIds.filter(Boolean)));
      if (uniqueIds.length === 0) return;

      setScanningIds((current) => {
        const next = new Set(current);
        uniqueIds.forEach((id) => next.add(id));
        return next;
      });

      try {
        await Promise.all(
          uniqueIds.map((repositoryId) =>
            scanMutation.mutateAsync({
              params: {
                path: {
                  id: repositoryId,
                },
              },
              body: {
                force: false,
              },
            }),
          ),
        );
        await invalidateRepositoryAwareQueries(queryClient);
      } finally {
        setScanningIds((current) => {
          const next = new Set(current);
          uniqueIds.forEach((id) => next.delete(id));
          return next;
        });
      }
    },
    [queryClient, scanMutation],
  );

  return useMemo(
    () => ({
      scanRepository,
      scanRepositories,
      scanningIds,
      isScanning: scanningIds.size > 0 || scanMutation.isPending,
    }),
    [
      scanMutation.isPending,
      scanRepositories,
      scanRepository,
      scanningIds,
    ],
  );
}

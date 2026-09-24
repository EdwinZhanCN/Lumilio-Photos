import { useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api, client } from "@/lib/http-commons/queryClient";
import { storageViewQueryKey } from "./useStorageView";

const ACTIVE_VERIFICATION_STATUSES = new Set(["queued", "crawling", "catching_up", "finalizing"]);

export function isActiveVerificationStatus(status?: string): boolean {
  return status != null && ACTIVE_VERIFICATION_STATUSES.has(status);
}

export function useRepositoryVerificationList(repositoryId: string | undefined, enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/repositories/{id}/verifications",
    {
      params: {
        path: { id: repositoryId ?? "" },
        query: { limit: 50, offset: 0 },
      },
    },
    { enabled: enabled && Boolean(repositoryId) },
  );
}

export function useRepositoryVerificationDetail(
  repositoryId: string | undefined,
  operationId: string | undefined,
  enabled: boolean,
) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/repositories/{id}/verifications/{operation_id}",
    {
      params: {
        path: {
          id: repositoryId ?? "",
          operation_id: operationId ?? "",
        },
      },
    },
    { enabled: enabled && Boolean(repositoryId) && Boolean(operationId) },
  );
}

export function useRepositoryVerificationCancel() {
  const queryClient = useQueryClient();
  const cancelMutation = $api.useMutation(
    "post",
    "/api/v1/storage/repositories/{id}/verifications/{operation_id}/cancel",
  );
  const [cancellingIds, setCancellingIds] = useState<Set<string>>(() => new Set());

  const cancelVerification = useCallback(
    async (repositoryId: string) => {
      setCancellingIds((current) => new Set(current).add(repositoryId));
      try {
        const latest = await client.GET("/api/v1/storage/repositories/{id}/verifications/latest", {
          params: { path: { id: repositoryId } },
        });
        const operationId = latest.data?.operation_id;
        if (!operationId || !isActiveVerificationStatus(latest.data?.status)) {
          throw new Error("No active verification operation.");
        }
        await cancelMutation.mutateAsync({
          params: {
            path: { id: repositoryId, operation_id: operationId },
          },
        });
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
          queryClient.invalidateQueries({
            queryKey: ["get", "/api/v1/storage/repositories/{id}/verifications/latest"],
          }),
          queryClient.invalidateQueries({
            queryKey: ["get", "/api/v1/storage/repositories/{id}/verifications"],
          }),
        ]);
      } finally {
        setCancellingIds((current) => {
          const next = new Set(current);
          next.delete(repositoryId);
          return next;
        });
      }
    },
    [cancelMutation, queryClient],
  );

  return useMemo(
    () => ({
      cancelVerification,
      cancellingIds,
      isCancelling: cancelMutation.isPending,
    }),
    [cancelVerification, cancelMutation.isPending, cancellingIds],
  );
}

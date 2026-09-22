import { useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

function isDuplicateQueryKey(queryKey: readonly unknown[]) {
  return (
    queryKey[0] === "get" &&
    (queryKey[1] === "/api/v1/duplicates/summary" || queryKey[1] === "/api/v1/duplicates/groups")
  );
}

export function useRepositoryDuplicateDetect() {
  const queryClient = useQueryClient();
  const detectMutation = $api.useMutation("post", "/api/v1/duplicates/detect");
  const [detectingIds, setDetectingIds] = useState<Set<string>>(() => new Set());

  const detectDuplicates = useCallback(
    async (repositoryId: string) => {
      setDetectingIds((current) => new Set(current).add(repositoryId));
      try {
        const result = await detectMutation.mutateAsync({
          body: { repository_id: repositoryId },
        });
        await queryClient.invalidateQueries({
          predicate: (query) => isDuplicateQueryKey(query.queryKey),
        });
        return result;
      } finally {
        setDetectingIds((current) => {
          const next = new Set(current);
          next.delete(repositoryId);
          return next;
        });
      }
    },
    [detectMutation, queryClient],
  );

  return useMemo(
    () => ({
      detectDuplicates,
      detectingIds,
      isDetecting: detectingIds.size > 0 || detectMutation.isPending,
    }),
    [detectDuplicates, detectMutation.isPending, detectingIds],
  );
}

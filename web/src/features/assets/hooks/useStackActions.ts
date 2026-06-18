import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export const useStackActions = () => {
  const queryClient = useQueryClient();
  const createManualStackMutation = $api.useMutation(
    "post",
    "/api/v1/assets/stacks",
  );

  const invalidateAssetAndStackQueries = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (!Array.isArray(key)) return false;

        const path = key[1];
        return (
          path === "/api/v1/assets/list" ||
          path === "/api/v1/assets/search" ||
          path === "/api/v1/assets/{id}/stack"
        );
      },
    });
  }, [queryClient]);

  const createStack = useCallback(
    async (assetIds: string[]) => {
      await createManualStackMutation.mutateAsync({
        body: { asset_ids: assetIds },
      });
      await invalidateAssetAndStackQueries();
    },
    [createManualStackMutation, invalidateAssetAndStackQueries],
  );

  return {
    createStack,
    isCreatingStack: createManualStackMutation.isPending,
  };
};

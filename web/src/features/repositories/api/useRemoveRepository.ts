import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { storageViewQueryKey } from "./useStorageView";

export function useRemoveRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/storage/repositories/{id}/detach");

  const removeRepository = useCallback(
    async (repositoryId: string, confirmationName: string) => {
      const response = await mutation.mutateAsync({
        params: { path: { id: repositoryId } },
        body: { confirmation_name: confirmationName },
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/targets"] }),
        queryClient.invalidateQueries({ queryKey: ["post", "/api/v1/assets/list"] }),
        queryClient.invalidateQueries({ queryKey: ["post", "/api/v1/assets/search"] }),
      ]);
      return response;
    },
    [mutation, queryClient],
  );

  return {
    removeRepository,
    isPending: mutation.isPending,
    error: mutation.error,
  };
}

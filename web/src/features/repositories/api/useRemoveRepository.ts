import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export function useRemoveRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("delete", "/api/v1/repositories/{id}");

  const removeRepository = useCallback(
    async (repositoryId: string, confirmationName: string) => {
      const response = await mutation.mutateAsync({
        params: { path: { id: repositoryId } },
        body: { confirmation_name: confirmationName },
      });
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/indexing/repositories"],
        }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] }),
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

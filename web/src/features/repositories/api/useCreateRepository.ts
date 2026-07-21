import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

type CreateRepositoryInput = {
  name: string;
  rootId?: string;
  cloudCredentialId?: string;
  role?: "primary" | "regular";
  storageStrategy?: "cas" | "date" | "flat";
  duplicateHandling?: "overwrite" | "rename" | "uuid";
};

export function useCreateRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/repositories");

  const createRepository = useCallback(
    async ({
      name,
      rootId,
      cloudCredentialId,
      role,
      storageStrategy,
      duplicateHandling,
    }: CreateRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: {
          name,
          root_id: rootId,
          cloud_credential_id: cloudCredentialId,
          role,
          storage_strategy: storageStrategy,
          duplicate_handling: duplicateHandling,
        },
      });

      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/indexing/repositories"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/repository-roots"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["post", "/api/v1/assets/list"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["post", "/api/v1/assets/search"],
        }),
      ]);

      return response;
    },
    [mutation, queryClient],
  );

  return {
    createRepository,
    isPending: mutation.isPending,
    error: mutation.error,
  };
}

import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

type CreateRepositoryInput = {
  name: string;
  cloudCredentialId?: string;
};

export function useCreateRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/repositories");

  const createRepository = useCallback(
    async ({ name, cloudCredentialId }: CreateRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: {
          name,
          cloud_credential_id: cloudCredentialId,
        },
      });

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

      return response;
    },
    [mutation, queryClient],
  );

  return {
    createRepository,
    isPending: mutation.isPending,
  };
}

import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { RepositoryStorageStrategy } from "../components/StorageStrategyPicker";

export type CreateRepositoryInput = {
  name: string;
  directoryName?: string;
  rootId?: string;
  role?: "primary" | "regular";
  storageStrategy: RepositoryStorageStrategy;
  riskConfirmation?: boolean;
};

export function buildCreateRepositoryRequestBody({
  name,
  directoryName,
  rootId,
  role,
  storageStrategy,
  riskConfirmation,
}: CreateRepositoryInput) {
  return {
    name,
    directory_name: directoryName,
    root_id: rootId,
    role,
    storage_strategy: storageStrategy,
    risk_confirmation: riskConfirmation,
  };
}

export function useCreateRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/repositories");

  const createRepository = useCallback(
    async ({
      name,
      directoryName,
      rootId,
      role,
      storageStrategy,
      riskConfirmation,
    }: CreateRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: buildCreateRepositoryRequestBody({
          name,
          directoryName,
          rootId,
          role,
          storageStrategy,
          riskConfirmation,
        }),
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

import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { RepositoryStorageStrategy } from "../components/StorageStrategyPicker";
import { storageViewQueryKey } from "./useStorageView";

export type CreateRepositoryInput = {
  name: string;
  directoryName?: string;
  storageLocationId?: string;
  role?: "primary" | "regular";
  storageStrategy: RepositoryStorageStrategy;
  riskConfirmation?: boolean;
};

export function buildCreateRepositoryRequestBody({
  name,
  directoryName,
  storageLocationId,
  role,
  storageStrategy,
  riskConfirmation,
}: CreateRepositoryInput) {
  return {
    name,
    directory_name: directoryName,
    storage_location_id: storageLocationId,
    role,
    storage_strategy: storageStrategy,
    risk_confirmation: riskConfirmation,
  };
}

export function useCreateRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/storage/repositories");

  const createRepository = useCallback(
    async ({
      name,
      directoryName,
      storageLocationId,
      role,
      storageStrategy,
      riskConfirmation,
    }: CreateRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: buildCreateRepositoryRequestBody({
          name,
          directoryName,
          storageLocationId,
          role,
          storageStrategy,
          riskConfirmation,
        }),
      });

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/targets"] }),
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

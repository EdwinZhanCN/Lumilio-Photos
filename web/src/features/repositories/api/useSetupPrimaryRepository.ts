import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { RepositoryStorageStrategy } from "../components/StorageStrategyPicker";

export type SetupPrimaryRepositoryInput = {
  name: string;
  storageStrategy: RepositoryStorageStrategy;
  riskConfirmation?: boolean;
};

export function buildSetupPrimaryRepositoryRequestBody({
  name,
  storageStrategy,
  riskConfirmation,
}: SetupPrimaryRepositoryInput) {
  return {
    name,
    storage_strategy: storageStrategy,
    risk_confirmation: riskConfirmation,
  };
}

export function useSetupPrimaryRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/setup/primary-repository");

  const createPrimaryRepository = useCallback(
    async ({ name, storageStrategy, riskConfirmation }: SetupPrimaryRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: buildSetupPrimaryRepositoryRequestBody({
          name,
          storageStrategy,
          riskConfirmation,
        }),
      });

      await Promise.all([
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
    createPrimaryRepository,
    isPending: mutation.isPending,
    error: mutation.error,
  };
}

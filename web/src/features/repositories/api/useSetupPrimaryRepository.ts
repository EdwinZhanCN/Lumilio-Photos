import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { RepositoryStorageStrategy } from "../components/StorageStrategyPicker";

export type SetupPrimaryRepositoryInput = {
  name: string;
  storageStrategy: RepositoryStorageStrategy;
  assessmentToken?: string;
  acknowledgedRisks?: readonly string[];
};

export function buildSetupPrimaryRepositoryRequestBody({
  name,
  storageStrategy,
  assessmentToken,
  acknowledgedRisks,
}: SetupPrimaryRepositoryInput) {
  return {
    name,
    storage_strategy: storageStrategy,
    assessment_token: assessmentToken,
    acknowledged_risks: acknowledgedRisks ? [...acknowledgedRisks] : undefined,
  };
}

export function useSetupPrimaryRepository() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/setup/primary-repository");

  const createPrimaryRepository = useCallback(
    async ({
      name,
      storageStrategy,
      assessmentToken,
      acknowledgedRisks,
    }: SetupPrimaryRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: buildSetupPrimaryRepositoryRequestBody({
          name,
          storageStrategy,
          assessmentToken,
          acknowledgedRisks,
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

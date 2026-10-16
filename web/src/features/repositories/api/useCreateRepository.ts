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
  /** Token of the assessment the acknowledgement answers; required when risks exist. */
  assessmentToken?: string;
  /** Every risk code the assessment reported, acknowledged for this command only. */
  acknowledgedRisks?: readonly string[];
};

export function buildCreateRepositoryRequestBody({
  name,
  directoryName,
  storageLocationId,
  role,
  storageStrategy,
  assessmentToken,
  acknowledgedRisks,
}: CreateRepositoryInput) {
  return {
    name,
    directory_name: directoryName,
    storage_location_id: storageLocationId,
    role,
    storage_strategy: storageStrategy,
    assessment_token: assessmentToken,
    acknowledged_risks: acknowledgedRisks ? [...acknowledgedRisks] : undefined,
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
      assessmentToken,
      acknowledgedRisks,
    }: CreateRepositoryInput) => {
      const response = await mutation.mutateAsync({
        body: buildCreateRepositoryRequestBody({
          name,
          directoryName,
          storageLocationId,
          role,
          storageStrategy,
          assessmentToken,
          acknowledgedRisks,
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

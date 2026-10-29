import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";
import { createUUID } from "@/lib/uuid";
import { storageViewQueryKey } from "./useStorageView";

export type RepositoryCandidate = components["schemas"]["dto.RepositoryCandidateDTO"];

/**
 * The consent a candidate command carries: the token of the Server's
 * open_repository review of that exact folder and the risk codes the
 * administrator acknowledged for it. A risky folder without them answers
 * storage/confirmation-required.
 */
export type CandidateConsent = { assessmentToken?: string; acknowledgedRisks?: readonly string[] };

export function useRepositoryCandidates(enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/candidates",
    {},
    { enabled, staleTime: 10_000, refetchOnWindowFocus: true },
  );
}

export function useOpenRepositoryCandidate() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/storage/candidates/open");

  const openRepositoryCandidate = useCallback(
    async (directoryName: string, consent: CandidateConsent = {}) => {
      const response = await mutation.mutateAsync({
        params: { header: { "Idempotency-Key": createUUID() } },
        body: {
          directory_name: directoryName,
          assessment_token: consent.assessmentToken,
          acknowledged_risks: consent.acknowledgedRisks
            ? [...consent.acknowledgedRisks]
            : undefined,
        },
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/candidates"] }),
        queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/targets"] }),
      ]);
      return response;
    },
    [mutation, queryClient],
  );

  return {
    openRepositoryCandidate,
    isPending: mutation.isPending,
    error: mutation.error,
  };
}

export function useResolveRepositoryCandidate() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/storage/candidates/resolve");

  const resolveRepositoryCandidate = useCallback(
    async (
      directoryName: string,
      resolution: "update_location" | "add_separate",
      consent: CandidateConsent = {},
    ) => {
      const response = await mutation.mutateAsync({
        params: { header: { "Idempotency-Key": createUUID() } },
        body: {
          directory_name: directoryName,
          resolution,
          assessment_token: consent.assessmentToken,
          acknowledged_risks: consent.acknowledgedRisks
            ? [...consent.acknowledgedRisks]
            : undefined,
        },
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/candidates"] }),
        queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/targets"] }),
      ]);
      return response;
    },
    [mutation, queryClient],
  );

  return {
    resolveRepositoryCandidate,
    isPending: mutation.isPending,
    error: mutation.error,
  };
}

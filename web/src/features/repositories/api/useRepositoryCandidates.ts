import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

export type RepositoryCandidate = components["schemas"]["dto.RepositoryCandidateDTO"];

export function useRepositoryCandidates(enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/repository-candidates",
    {},
    { enabled, staleTime: 10_000, refetchOnWindowFocus: true },
  );
}

export function useOpenRepositoryCandidate() {
  const queryClient = useQueryClient();
  const mutation = $api.useMutation("post", "/api/v1/repository-candidates/open");

  const openRepositoryCandidate = useCallback(
    async (directoryName: string, riskConfirmation = false) => {
      const response = await mutation.mutateAsync({
        params: { header: { "Idempotency-Key": crypto.randomUUID() } },
        body: { directory_name: directoryName, risk_confirmation: riskConfirmation },
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repository-candidates"] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/indexing/repositories"],
        }),
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
  const mutation = $api.useMutation("post", "/api/v1/repository-candidates/resolve");

  const resolveRepositoryCandidate = useCallback(
    async (
      directoryName: string,
      resolution: "update_location" | "add_separate",
      riskConfirmation = false,
    ) => {
      const response = await mutation.mutateAsync({
        params: { header: { "Idempotency-Key": crypto.randomUUID() } },
        body: { directory_name: directoryName, resolution, risk_confirmation: riskConfirmation },
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repository-candidates"] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/indexing/repositories"],
        }),
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

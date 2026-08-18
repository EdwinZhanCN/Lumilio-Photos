import { useMutation, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { authenticatedFetch } from "@/lib/http-commons/client";
import { normalizeProblem, readProblemResponse } from "@/lib/http-commons/problem";
import type { Schemas } from "./types";

export type SystemSettings = Schemas["dto.SystemSettingsDTO"];
export type UpdateSystemSettings = Schemas["dto.UpdateSystemSettingsDTO"];
export type ValidateLLMDraft = Schemas["dto.ValidateLLMSettingsRequestDTO"];

export const systemSettingsQueryKey = ["get", "/api/v1/settings/system"] as const;

export function useSystemSettings(): UseQueryResult<SystemSettings, unknown> {
  return $api.useQuery(
    "get",
    "/api/v1/settings/system",
    {},
    {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<SystemSettings, unknown>;
}

export function useUpdateSystemSettings() {
  const queryClient = useQueryClient();
  return $api.useMutation("patch", "/api/v1/settings/system", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: systemSettingsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/setup/status"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/capabilities"],
        }),
      ]);
    },
  });
}

export async function validateLLMDraft(body: ValidateLLMDraft): Promise<void> {
  const response = await authenticatedFetch("/api/v1/settings/system/validate-llm", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) throw await readProblemResponse(response);
  const payload: unknown = await response.json().catch(() => undefined);
  const valid = Boolean(
    payload && typeof payload === "object" && "valid" in payload && payload.valid,
  );
  if (!valid) throw normalizeProblem(undefined);
}

export function useValidateLLMSettings() {
  return useMutation({ mutationFn: validateLLMDraft });
}

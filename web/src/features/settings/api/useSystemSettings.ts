import { useMutation, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { authenticatedFetch } from "@/lib/http-commons/client";
import type { Schemas } from "./types";

export type SystemSettings = Schemas["dto.SystemSettingsDTO"];

type GeneratedUpdateSystemSettings = Schemas["dto.UpdateSystemSettingsDTO"];
type GeneratedUpdateLLM = NonNullable<GeneratedUpdateSystemSettings["llm"]>;

// OpenAPI annotations in the Server already include the explicit `none`
// sentinel. This source type keeps the checked-in client buildable until
// `task dto` regenerates schema.d.ts during the real gate run.
export type UpdateSystemSettings = Omit<GeneratedUpdateSystemSettings, "llm"> & {
  llm?: Omit<GeneratedUpdateLLM, "provider"> & {
    provider?: "none" | "ark" | "openai" | "deepseek" | "ollama";
  };
};

export interface ValidateLLMDraft {
  provider: "ark" | "openai" | "deepseek" | "ollama";
  model_name: string;
  base_url?: string;
  api_key?: string;
  use_stored_api_key?: boolean;
}

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
  return useMutation({
    mutationFn: async ({ body }: { body: UpdateSystemSettings }) => {
      const response = await authenticatedFetch("/api/v1/settings/system", {
        method: "PATCH",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(body),
      });
      const payload: unknown = await response.json().catch(() => undefined);
      if (!response.ok) {
        const detail = payload as { message?: string; error?: string } | undefined;
        throw new Error(detail?.message ?? detail?.error ?? `HTTP ${response.status}`);
      }
      return payload as SystemSettings;
    },
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
  const payload: unknown = await response.json().catch(() => undefined);
  const valid = Boolean(payload && typeof payload === "object" && "valid" in payload && payload.valid);
  if (!response.ok || !valid) {
    const detail = payload as { message?: string; error?: string } | undefined;
    throw new Error(detail?.message ?? detail?.error ?? `HTTP ${response.status}`);
  }
}

export function useValidateLLMSettings() {
  return useMutation({ mutationFn: validateLLMDraft });
}

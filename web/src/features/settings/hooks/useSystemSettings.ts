import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, Schemas } from "../api-types";

export type SystemSettings = Schemas["dto.SystemSettingsDTO"];
export type UpdateSystemSettings = Schemas["dto.UpdateSystemSettingsDTO"];

export const systemSettingsQueryKey = [
  "get",
  "/api/v1/settings/system",
] as const;

export function useSystemSettings(): UseQueryResult<
  ApiResult<SystemSettings>,
  unknown
> {
  return $api.useQuery(
    "get",
    "/api/v1/settings/system",
    {},
    {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<SystemSettings>, unknown>;
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

export function useValidateLLMSettings() {
  return $api.useMutation("post", "/api/v1/settings/system/validate-llm");
}

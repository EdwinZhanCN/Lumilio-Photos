import type { UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, Schemas } from "../api-types";

export type RuntimeInfo = Schemas["dto.RuntimeInfoDTO"];

export const runtimeInfoQueryKey = [
  "get",
  "/api/v1/settings/runtime-info",
] as const;

export function useRuntimeInfo(): UseQueryResult<
  ApiResult<RuntimeInfo>,
  unknown
> {
  return $api.useQuery(
    "get",
    "/api/v1/settings/runtime-info",
    {},
    {
      staleTime: Infinity,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<RuntimeInfo>, unknown>;
}

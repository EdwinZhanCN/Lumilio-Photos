import type { UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, BootstrapStatus } from "../auth.type.ts";

export function useBootstrapStatus(): UseQueryResult<
  ApiResult<BootstrapStatus>,
  unknown
> {
  return $api.useQuery(
    "get",
    "/api/v1/auth/bootstrap-status",
    {},
    {
      staleTime: 10_000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<BootstrapStatus>, unknown>;
}

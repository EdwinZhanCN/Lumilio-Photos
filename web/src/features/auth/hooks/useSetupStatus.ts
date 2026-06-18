import type { UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

export type SetupStatus = components["schemas"]["dto.SetupStatusDTO"];

export const setupStatusQueryKey = ["get", "/api/v1/setup/status"] as const;

export function useSetupStatus(): UseQueryResult<SetupStatus, unknown> {
  return $api.useQuery(
    "get",
    "/api/v1/setup/status",
    {},
    {
      staleTime: 10_000,
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<SetupStatus, unknown>;
}

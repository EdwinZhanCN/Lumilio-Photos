import type { UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

export type SetupPrimaryTarget = components["schemas"]["dto.SetupPrimaryTargetDTO"];
export type SetupPrimaryReview = components["schemas"]["dto.SetupPrimaryReviewDTO"];
export type SetupDefaultStorage = components["schemas"]["dto.SetupDefaultStorageDTO"];

export const setupPrimaryTargetQueryKey = ["get", "/api/v1/setup/primary-target"] as const;

/**
 * The authenticated administrator's review of the Primary Repository setup:
 * the Default Storage Location's state and what its primary/ folder holds,
 * with the actions setup may take. Public setup status never carries these.
 */
export function useSetupPrimaryTarget(enabled = true): UseQueryResult<SetupPrimaryTarget, unknown> {
  return $api.useQuery(
    "get",
    "/api/v1/setup/primary-target",
    {},
    { enabled, staleTime: 10_000, refetchOnWindowFocus: false },
  ) as UseQueryResult<SetupPrimaryTarget, unknown>;
}

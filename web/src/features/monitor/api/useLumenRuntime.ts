import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

export type LumenRuntime = components["schemas"]["dto.LumenRuntimeDTO"];
export type LumenBackendStatus = components["schemas"]["dto.LumenBackendStatusDTO"];
export type LumenNodeRuntime = components["schemas"]["dto.LumenNodeRuntimeDTO"];

export function useLumenRuntime(refetchInterval: number | false = 5000) {
  return $api.useQuery(
    "get",
    "/api/v1/admin/lumen/runtime",
    {},
    {
      refetchInterval,
    },
  );
}

import { $api } from "@/lib/http-commons/queryClient";

/** One shared read for Catalog work and queue delivery diagnostics. */
export function useProcessingMonitor() {
  return $api.useQuery(
    "get",
    "/api/v1/admin/monitor/processing",
    { params: { query: { error_limit: 5 } } },
    { staleTime: 5000, refetchInterval: 5000, refetchIntervalInBackground: true, retry: false },
  );
}

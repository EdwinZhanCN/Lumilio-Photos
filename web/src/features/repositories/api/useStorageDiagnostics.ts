import { $api } from "@/lib/http-commons/queryClient";
import { normalizeStorageDiagnosticsResponse } from "../model/storageEntities";

export function useStorageDiagnostics(enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/diagnostics",
    {},
    { enabled, staleTime: 15_000, select: normalizeStorageDiagnosticsResponse },
  );
}

export function useLifecycleAudit(enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/audit",
    { params: { query: { limit: 100, offset: 0 } } },
    { enabled, staleTime: 10_000 },
  );
}

export function useStorageSupportBundle() {
  return $api.useQuery(
    "get",
    "/api/v1/storage/support-bundle",
    {},
    { enabled: false, staleTime: 0 },
  );
}

import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

export type ProcessingSummary = Schemas["processing.Summary"];
export type ProcessingStage = Schemas["processing.StageSummary"];
export type ProcessingStageId = NonNullable<ProcessingStage["id"]>;
export type ProcessingItem = Schemas["processing.Item"];
export type ProcessingItemState = "failed" | "queued";

const SUMMARY_KEY = ["get", "/api/v1/admin/processing"] as const;
const ITEMS_KEY = ["get", "/api/v1/admin/processing/stages/{stage}/items"] as const;

/** One five-second read of every stage card and the overview. */
export function useProcessingSummary() {
  return $api.useQuery(
    "get",
    "/api/v1/admin/processing",
    {},
    {
      staleTime: 5000,
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      retry: false,
    },
  );
}

/** One bounded page of a stage's failed or queued subjects. */
export function useProcessingStageItems(
  stage: ProcessingStageId | undefined,
  state: ProcessingItemState,
  limit: number,
) {
  return $api.useQuery(
    "get",
    "/api/v1/admin/processing/stages/{stage}/items",
    { params: { path: { stage: stage ?? "import" }, query: { state, limit } } },
    { enabled: Boolean(stage), refetchInterval: 5000, retry: false },
  );
}

function useInvalidateProcessing() {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: SUMMARY_KEY }),
      queryClient.invalidateQueries({ queryKey: ITEMS_KEY }),
    ]);
}

/** Re-requests a stage's failures through the Catalog. */
export function useRetryProcessingStage() {
  const invalidate = useInvalidateProcessing();
  return $api.useMutation("post", "/api/v1/admin/processing/stages/{stage}/retry", {
    onSuccess: invalidate,
  });
}

/** Re-requests one file's stage through the existing per-asset path. */
export function useRetryProcessingItem() {
  const invalidate = useInvalidateProcessing();
  return $api.useMutation("post", "/api/v1/assets/{id}/reprocess", { onSuccess: invalidate });
}

/** Disposable delivery diagnostics, fetched only while the dialog is open. */
export function useProcessingDiagnostics(enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/admin/processing/diagnostics",
    { params: { query: { error_limit: 5 } } },
    { enabled, refetchInterval: enabled ? 5000 : false, retry: false },
  );
}

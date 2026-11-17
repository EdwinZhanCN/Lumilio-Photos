import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

export type StorageAssessment = components["schemas"]["dto.StorageAssessmentDTO"];
export type StorageAssessmentIntent =
  components["schemas"]["dto.StorageAssessmentRequestDTO"]["intent"];

/**
 * Read-only Server review of a Repository create target: support, typed
 * refusal, the risks to acknowledge, and the consent token the create command
 * binds those acknowledgements to. The query key is the target itself, so a
 * different Storage Location or folder is a different review and the token a
 * component holds can never describe another target.
 */
export function useStorageAssessment(
  input: { intent: StorageAssessmentIntent; storageLocationId?: string; directoryName?: string },
  enabled: boolean,
) {
  return $api.useQuery(
    "post",
    "/api/v1/storage/assessments",
    {
      body: {
        intent: input.intent,
        storage_location_id: input.storageLocationId,
        directory_name: input.directoryName,
      },
    },
    {
      enabled,
      // A review is only as fresh as the disk; never serve a cached token for a
      // reopened dialog, and never retry a typed refusal.
      staleTime: 0,
      gcTime: 0,
      retry: false,
      refetchOnWindowFocus: false,
    },
  );
}

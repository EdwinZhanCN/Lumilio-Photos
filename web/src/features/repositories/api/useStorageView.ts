import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

export type StorageViewResponse = components["schemas"]["dto.StorageViewResponseDTO"];

export const storageViewQueryKey = ["get", "/api/v1/storage/view"] as const;

export const storageViewQueryOptions = {
  staleTime: 30_000,
  refetchOnWindowFocus: true,
} as const;

export function useStorageView() {
  return $api.useQuery("get", "/api/v1/storage/view", {}, storageViewQueryOptions);
}

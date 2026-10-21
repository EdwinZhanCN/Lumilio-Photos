import { $api } from "@/lib/http-commons/queryClient";
import { normalizeStorageViewLocations } from "../model/storageEntities";
import { storageViewQueryOptions } from "./useStorageView";

export { storageViewQueryKey, useStorageView, type StorageViewResponse } from "./useStorageView";

export function useStorageLocations() {
  return $api.useQuery(
    "get",
    "/api/v1/storage/view",
    {},
    {
      ...storageViewQueryOptions,
      select: normalizeStorageViewLocations,
    },
  );
}

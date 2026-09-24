import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { storageViewQueryKey } from "./useStorageView";

export function useRemoveStorageLocation() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/storage/locations/{id}/detach", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: [...storageViewQueryKey] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/storage/targets"] }),
      ]);
    },
  });
}

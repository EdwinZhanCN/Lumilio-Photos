import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export function useRemoveStorageLocation() {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/repository-roots/{id}", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repository-roots"] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] }),
      ]);
    },
  });
}

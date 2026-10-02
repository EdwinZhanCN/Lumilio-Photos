import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

/** Shared lifecycle commands invalidate server projections even after a partial purge. */
export function useAssetLifecycle() {
  const client = useQueryClient();
  const onSettled = async () => {
    await client.invalidateQueries({
      predicate: (query) => {
        const path = query.queryKey[1];
        return (
          typeof path === "string" &&
          /^\/api\/v1\/(assets|albums|people|events|music|storage|repositories|tags|locations|share-links)(\/|$)/.test(
            path,
          )
        );
      },
    });
  };
  const trash = $api.useMutation("post", "/api/v1/assets/trash", { onSettled });
  const restore = $api.useMutation("post", "/api/v1/assets/restore", { onSettled });
  const permanentlyDelete = $api.useMutation("post", "/api/v1/assets/delete-permanently", {
    onSettled,
  });
  const removeMissing = $api.useMutation("post", "/api/v1/assets/remove-missing", { onSettled });
  const emptyTrash = $api.useMutation("post", "/api/v1/assets/empty-trash", { onSettled });
  return { trash, restore, permanentlyDelete, removeMissing, emptyTrash };
}

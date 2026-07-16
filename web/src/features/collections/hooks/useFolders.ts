import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons";

export type FolderListResponse = components["schemas"]["dto.FolderListResponseDTO"];
export type FolderSummary = components["schemas"]["dto.FolderSummaryDTO"];

/**
 * Lists immediate child folders under `parentPath` (repository-relative,
 * "" for root). Counts/covers are recursive over descendants.
 */
export function useFolders(repositoryId: string | undefined, parentPath: string) {
  return $api.useQuery("get", "/api/v1/assets/folders", {
    params: {
      query: {
        repository_id: repositoryId,
        path: parentPath,
      },
    },
  });
}

/**
 * Aggregate stats (recursive) for exactly one folder path, used for the
 * Folder detail header. Requires a resolved repository ID.
 */
export function useFolderSummary(repositoryId: string | undefined, folderPath: string) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/folders/summary",
    {
      params: {
        query: {
          repository_id: repositoryId ?? "",
          path: folderPath,
        },
      },
    },
    {
      enabled: Boolean(repositoryId),
    },
  );
}

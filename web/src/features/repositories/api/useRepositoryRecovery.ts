import { useQueryClient } from "@tanstack/react-query";
import { getToken } from "@/lib/http-commons/auth";
import { baseUrl } from "@/lib/http-commons/client";
import { readProblemResponse } from "@/lib/http-commons/problem";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";
import { storageViewQueryKey } from "./useStorageView";

export type RepositoryRecovery = components["schemas"]["dto.RepositoryRecoveryDTO"];
export type RepositoryRecoveryItem = components["schemas"]["dto.RepositoryRecoveryItemDTO"];
export type RepositoryRecoveryReceipt = components["schemas"]["dto.RepositoryRecoveryReceiptDTO"];

export const repositoryRecoveryQueryKey = (repositoryId: string) =>
  [
    "get",
    "/api/v1/storage/repositories/{id}/recovery",
    { params: { path: { id: repositoryId } } },
  ] as const;

/** The private-recovery inventory of one Repository: receipts, items, held Trash. */
export function useRepositoryRecovery(repositoryId: string | undefined, enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/storage/repositories/{id}/recovery",
    { params: { path: { id: repositoryId ?? "" } } },
    { enabled: enabled && Boolean(repositoryId), staleTime: 10_000 },
  );
}

/** Explicit removal of one preserved item; the inventory and Storage view refresh. */
export function useDeleteRecoveryItem(repositoryId: string | undefined) {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/storage/repositories/{id}/recovery/items/{item}", {
    onSettled: async () => {
      if (repositoryId) {
        await queryClient.invalidateQueries({ queryKey: repositoryRecoveryQueryKey(repositoryId) });
      }
      await queryClient.invalidateQueries({ queryKey: storageViewQueryKey });
    },
  });
}

export function recoveryItemContentPath(repositoryId: string, itemId: string): string {
  return `/api/v1/storage/repositories/${encodeURIComponent(repositoryId)}/recovery/items/${encodeURIComponent(itemId)}/content`;
}

export function recoveryItemFilename(item: Pick<RepositoryRecoveryItem, "path">): string {
  const segments = (item.path ?? "").split("/").filter(Boolean);
  return segments[segments.length - 1] ?? "recovery-item";
}

/**
 * Downloads one item's preserved bytes through the authenticated content
 * endpoint and hands them to the browser as a file; the item is unchanged.
 */
export async function exportRecoveryItem(
  repositoryId: string,
  item: Pick<RepositoryRecoveryItem, "id" | "path">,
  fetcher: typeof fetch = fetch,
): Promise<void> {
  const headers = new Headers();
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetcher(
    `${baseUrl}${recoveryItemContentPath(repositoryId, item.id ?? "")}`,
    {
      headers,
    },
  );
  if (!response.ok) throw await readProblemResponse(response);
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = recoveryItemFilename(item);
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

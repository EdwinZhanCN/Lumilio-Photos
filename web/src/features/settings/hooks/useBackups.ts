import { useQueryClient } from "@tanstack/react-query";
import { $api, client } from "@/lib/http-commons/queryClient";
import type { Schemas } from "../api-types";

export type BackupEntry = Schemas["dto.BackupEntryDTO"];

export const backupsQueryKey = ["get", "/api/v1/settings/backups"] as const;

/** Lists database dumps, newest first. Pass poll=true to refetch every few
 * seconds (used briefly after "back up now", whose dump appears when the
 * background job finishes). */
export function useBackups(poll = false) {
  return $api.useQuery(
    "get",
    "/api/v1/settings/backups",
    {},
    {
      refetchOnWindowFocus: false,
      refetchInterval: poll ? 3000 : false,
    },
  );
}

export function useCreateBackup() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/settings/backups", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: backupsQueryKey });
    },
  });
}

export function useDeleteBackup() {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/settings/backups/{name}", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: backupsQueryKey });
    },
  });
}

/** Restoring replaces the whole database; the caller is expected to reload the
 * app after success — every cached query is stale by definition. */
export function useRestoreBackup() {
  return $api.useMutation("post", "/api/v1/settings/backups/{name}/restore");
}

/** Authenticated blob download (a plain <a href> would miss the bearer token).
 * Mirrors the triggerDownload pattern used by bulk asset downloads. */
export async function downloadBackup(name: string): Promise<void> {
  const { data, error } = await client.GET("/api/v1/settings/backups/{name}/download", {
    params: { path: { name } },
    parseAs: "blob",
  });
  if (error || !(data instanceof Blob)) {
    throw new Error("backup download failed");
  }
  const blobUrl = window.URL.createObjectURL(data);
  const link = document.createElement("a");
  link.href = blobUrl;
  link.setAttribute("download", name);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(blobUrl);
}

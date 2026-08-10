import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { $api, client } from "@/lib/http-commons/queryClient";
import { authenticatedFetch } from "@/lib/http-commons/client";
import type { Schemas } from "./types";

export type BackupEntry = Schemas["dto.BackupEntryDTO"];

export interface RestoreOperation {
  id: string;
  backup_name: string;
  status:
    | "staged"
    | "restart_requested"
    | "installing"
    | "verifying"
    | "completed"
    | "rolling_back"
    | "rolled_back"
    | "failed";
  message: string;
  error_code?: string;
  restore_point?: string;
  requested_at: string;
  updated_at: string;
  completed_at?: string;
}

export const backupsQueryKey = ["get", "/api/v1/settings/backups"] as const;

const parseResponse = async <T>(response: Response): Promise<T> => {
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) {
    const detail = payload as { message?: string; error?: string } | undefined;
    throw new Error(detail?.message ?? detail?.error ?? `HTTP ${response.status}`);
  }
  return payload as T;
};

/** Lists SQLite snapshots, newest first. Pass poll=true to refetch every few
 * seconds after a queued snapshot request. */
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

/** Accepts a restore and returns the durable operation receipt before the
 * current Server generation restarts. */
export function useRestoreBackup() {
  return useMutation({
    mutationFn: async (name: string) => {
      const response = await authenticatedFetch(
        `/api/v1/settings/backups/${encodeURIComponent(name)}/restore`,
        { method: "POST", headers: { Accept: "application/json" } },
      );
      return parseResponse<RestoreOperation>(response);
    },
  });
}

/** Returns the latest durable restore receipt so a page reload can resume an
 * in-flight operation without relying on browser-only state. */
export function useLatestRestoreOperation(enabled = true) {
  return useQuery({
    queryKey: ["restore-operation", "latest"],
    enabled,
    retry: false,
    queryFn: async () => {
      const response = await authenticatedFetch("/api/v1/settings/backup-restores/latest", {
        headers: { Accept: "application/json" },
      });
      if (response.status === 404) return null;
      return parseResponse<RestoreOperation>(response);
    },
  });
}

/** Polls the durable receipt across temporary disconnects while the Server
 * swaps, verifies, or rolls back the database. */
export function useRestoreOperation(operationID: string | null) {
  return useQuery({
    queryKey: ["restore-operation", operationID],
    enabled: Boolean(operationID),
    retry: true,
    retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 5000),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "completed" || status === "rolled_back" || status === "failed"
        ? false
        : 1500;
    },
    queryFn: async () => {
      const response = await authenticatedFetch(
        `/api/v1/settings/backup-restores/${encodeURIComponent(operationID ?? "")}`,
        { headers: { Accept: "application/json" } },
      );
      return parseResponse<RestoreOperation>(response);
    },
  });
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

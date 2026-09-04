import type { components } from "@/lib/http-commons/schema.d.ts";
import { client } from "@/lib/http-commons/queryClient";
import { getToken } from "@/lib/http-commons/auth";
import { fetchEventSource } from "@microsoft/fetch-event-source";
import { normalizeProblem, readProblemResponse } from "@/lib/http-commons/problem";

type UploadOperationStatus = components["schemas"]["dto.UploadOperationStatusDTO"];

export interface WaitForUploadOperationsOptions {
  intervalMs?: number;
  timeoutMs?: number;
  signal?: AbortSignal;
  onUpdate?: (operation: UploadOperationStatus) => void;
}

/** Backend operation status endpoints accept at most this many receipt IDs. */
export const UPLOAD_RECEIPT_ID_LIMIT = 100;

export const chunkUploadReceiptIds = (
  receiptIds: string[],
  limit: number = UPLOAD_RECEIPT_ID_LIMIT,
): string[][] => {
  const ids = Array.from(new Set(receiptIds));
  if (ids.length === 0) return [];
  const chunks: string[][] = [];
  for (let index = 0; index < ids.length; index += limit) {
    chunks.push(ids.slice(index, index + limit));
  }
  return chunks;
};

const wait = (durationMs: number, signal?: AbortSignal): Promise<void> =>
  new Promise((resolve, reject) => {
    const timeout = globalThis.setTimeout(resolve, durationMs);
    signal?.addEventListener(
      "abort",
      () => {
        globalThis.clearTimeout(timeout);
        reject(new DOMException("Upload status polling aborted", "AbortError"));
      },
      { once: true },
    );
  });

/** Waits until every accepted upload reaches a backend ingest terminal state. */
async function pollUploadOperations(
  receiptIds: string[],
  options: WaitForUploadOperationsOptions = {},
): Promise<UploadOperationStatus[]> {
  const ids = Array.from(new Set(receiptIds));
  if (ids.length === 0) return [];

  const intervalMs = options.intervalMs ?? 750;
  const timeoutMs = options.timeoutMs ?? 10 * 60 * 1000;
  const deadline = Date.now() + timeoutMs;

  while (Date.now() <= deadline) {
    options.signal?.throwIfAborted();
    const { data, error } = await client.GET("/api/v1/assets/batch/operations", {
      params: { query: { receipt_ids: ids.join(",") } },
      signal: options.signal,
    });
    if (error) throw normalizeProblem(error);

    const operations = data?.operations ?? [];
    operations.forEach((operation) => options.onUpdate?.(operation));
    const byId = new Map(
      operations
        .filter(
          (operation): operation is UploadOperationStatus & { receipt_id: string } =>
            typeof operation.receipt_id === "string",
        )
        .map((operation) => [operation.receipt_id, operation] as const),
    );
    if (ids.every((id) => byId.get(id)?.terminal)) {
      return ids.map((id) => byId.get(id)!);
    }
    await wait(intervalMs, options.signal);
  }

  throw new Error("Timed out waiting for uploaded files to finish processing");
}

async function streamUploadOperations(
  ids: string[],
  options: WaitForUploadOperationsOptions,
): Promise<UploadOperationStatus[]> {
  const latest = new Map<string, UploadOperationStatus>();
  const baseUrl = import.meta.env.VITE_API_URL ?? "";
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  await fetchEventSource(
    `${baseUrl}/api/v1/assets/batch/operations/stream?receipt_ids=${ids.join(",")}`,
    {
      headers,
      signal: options.signal,
      openWhenHidden: true,
      onopen: async (response) => {
        if (!response.ok) throw await readProblemResponse(response);
      },
      onmessage: (message) => {
        if (message.event !== "operations" && message.event !== "done") return;
        const payload = JSON.parse(message.data) as { operations?: UploadOperationStatus[] };
        for (const operation of payload.operations ?? []) {
          if (typeof operation.receipt_id === "string") latest.set(operation.receipt_id, operation);
          options.onUpdate?.(operation);
        }
        if (message.event === "done") throw new UploadStreamComplete();
      },
      onerror: (error) => {
        throw error;
      },
    },
  ).catch((error: unknown) => {
    if (!(error instanceof UploadStreamComplete)) throw error;
  });
  const operations = ids
    .map((id) => latest.get(id))
    .filter((operation): operation is UploadOperationStatus => Boolean(operation));
  if (operations.length !== ids.length || !operations.every((operation) => operation.terminal)) {
    throw normalizeProblem(undefined);
  }
  return operations;
}

class UploadStreamComplete extends Error {}

async function waitForUploadOperationsBatch(
  receiptIds: string[],
  options: WaitForUploadOperationsOptions = {},
): Promise<UploadOperationStatus[]> {
  const ids = Array.from(new Set(receiptIds));
  if (ids.length === 0) return [];
  try {
    return await streamUploadOperations(ids, options);
  } catch (error) {
    if (options.signal?.aborted) throw error;
    return pollUploadOperations(ids, options);
  }
}

/** Uses SSE first and falls back to operation polling if streaming is unavailable. */
export async function waitForUploadOperations(
  receiptIds: string[],
  options: WaitForUploadOperationsOptions = {},
): Promise<UploadOperationStatus[]> {
  const batches = chunkUploadReceiptIds(receiptIds);
  if (batches.length === 0) return [];
  if (batches.length === 1) return waitForUploadOperationsBatch(batches[0], options);

  const settled = await Promise.allSettled(
    batches.map((batch) => waitForUploadOperationsBatch(batch, options)),
  );
  const operations: UploadOperationStatus[] = [];
  let firstError: unknown;
  for (const result of settled) {
    if (result.status === "fulfilled") {
      operations.push(...result.value);
      continue;
    }
    firstError ??= result.reason;
  }
  if (firstError) throw firstError;
  return operations;
}

import type { components } from "@/lib/http-commons/schema.d.ts";
import { client } from "@/lib/http-commons/queryClient";

type UploadJobStatus = components["schemas"]["dto.UploadJobStatusDTO"];

export interface WaitForUploadJobsOptions {
  intervalMs?: number;
  timeoutMs?: number;
  signal?: AbortSignal;
  onUpdate?: (job: UploadJobStatus) => void;
}

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
export async function waitForUploadJobs(
  taskIds: number[],
  options: WaitForUploadJobsOptions = {},
): Promise<UploadJobStatus[]> {
  const ids = Array.from(new Set(taskIds));
  if (ids.length === 0) return [];

  const intervalMs = options.intervalMs ?? 750;
  const timeoutMs = options.timeoutMs ?? 10 * 60 * 1000;
  const deadline = Date.now() + timeoutMs;

  while (Date.now() <= deadline) {
    options.signal?.throwIfAborted();
    const { data, error } = await client.GET("/api/v1/assets/batch/jobs", {
      params: { query: { task_ids: ids.join(",") } },
      signal: options.signal,
    });
    if (error) throw new Error(error.error || error.message || "Failed to load upload status");

    const jobs = data?.jobs ?? [];
    jobs.forEach((job) => options.onUpdate?.(job));
    if (jobs.length === ids.length && jobs.every((job) => job.terminal)) return jobs;
    await wait(intervalMs, options.signal);
  }

  throw new Error("Timed out waiting for uploaded files to finish processing");
}

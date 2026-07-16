import type { components } from "@/lib/http-commons/schema.d.ts";
import { client } from "@/lib/http-commons/queryClient";
import { getToken } from "@/lib/http-commons/auth";
import { fetchEventSource } from "@microsoft/fetch-event-source";

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
async function pollUploadJobs(
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

async function streamUploadJobs(
  ids: number[],
  options: WaitForUploadJobsOptions,
): Promise<UploadJobStatus[]> {
  const latest = new Map<number, UploadJobStatus>();
  const baseUrl = import.meta.env.VITE_API_URL ?? "";
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  await fetchEventSource(`${baseUrl}/api/v1/assets/batch/jobs/stream?task_ids=${ids.join(",")}`, {
    headers,
    signal: options.signal,
    openWhenHidden: true,
    onopen: async (response) => {
      if (!response.ok)
        throw new Error(`Upload status stream failed with status ${response.status}`);
    },
    onmessage: (message) => {
      if (message.event !== "jobs" && message.event !== "done") return;
      const payload = JSON.parse(message.data) as { jobs?: UploadJobStatus[] };
      for (const job of payload.jobs ?? []) {
        if (typeof job.task_id === "number") latest.set(job.task_id, job);
        options.onUpdate?.(job);
      }
      if (message.event === "done") throw new UploadStreamComplete();
    },
    onerror: (error) => {
      throw error;
    },
  }).catch((error: unknown) => {
    if (!(error instanceof UploadStreamComplete)) throw error;
  });
  const jobs = ids
    .map((id) => latest.get(id))
    .filter((job): job is UploadJobStatus => Boolean(job));
  if (jobs.length !== ids.length || !jobs.every((job) => job.terminal))
    throw new Error("Upload status stream ended early");
  return jobs;
}

class UploadStreamComplete extends Error {}

/** Uses SSE first and falls back to /batch/jobs polling if streaming is unavailable. */
export async function waitForUploadJobs(
  taskIds: number[],
  options: WaitForUploadJobsOptions = {},
): Promise<UploadJobStatus[]> {
  const ids = Array.from(new Set(taskIds));
  if (ids.length === 0) return [];
  try {
    return await streamUploadJobs(ids, options);
  } catch (error) {
    if (options.signal?.aborted) throw error;
    return pollUploadJobs(ids, options);
  }
}

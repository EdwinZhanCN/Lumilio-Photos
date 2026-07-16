import { client } from "@/lib/http-commons/queryClient";

const TERMINAL_SCAN_STATUSES = new Set(["completed", "failed", "cancelled"]);

const wait = (durationMs: number) =>
  new Promise<void>((resolve) => globalThis.setTimeout(resolve, durationMs));

export const waitForRepositoryScan = async (
  repositoryId: string,
  requestedAt: number,
  options: { intervalMs?: number; timeoutMs?: number } = {},
) => {
  const deadline = Date.now() + (options.timeoutMs ?? 10 * 60 * 1000);
  while (Date.now() <= deadline) {
    const { data, error, response } = await client.GET("/api/v1/repositories/{id}/scans/latest", {
      params: { path: { id: repositoryId } },
    });
    if (error && response.status !== 404) {
      throw new Error(error.error || error.message || "Failed to load repository scan status");
    }
    const startedAt = data?.started_at ? Date.parse(data.started_at) : 0;
    const belongsToRequest = startedAt >= requestedAt - 2_000;
    if (belongsToRequest && data?.status && TERMINAL_SCAN_STATUSES.has(data.status)) {
      if (data.status !== "completed") {
        throw new Error(data.error || `Repository scan ${data.status}`);
      }
      return data;
    }
    await wait(options.intervalMs ?? 750);
  }
  throw new Error("Timed out waiting for repository scan completion");
};

import { client } from "@/lib/http-commons/queryClient";
import { normalizeProblem, normalizeProblemReference } from "@/lib/http-commons/problem";

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
      throw normalizeProblem(error);
    }
    const startedAt = data?.started_at ? Date.parse(data.started_at) : 0;
    const belongsToRequest = startedAt >= requestedAt - 2_000;
    if (belongsToRequest && data?.status && TERMINAL_SCAN_STATUSES.has(data.status)) {
      if (data.status !== "completed") {
        throw normalizeProblemReference(data.problem) ?? normalizeProblem(undefined);
      }
      return data;
    }
    await wait(options.intervalMs ?? 750);
  }
  throw normalizeProblem(undefined);
};

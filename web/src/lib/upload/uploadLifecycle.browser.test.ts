import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vite-plus/test";
import type { components } from "@/lib/http-commons/schema.d.ts";

type UploadJobStatus = components["schemas"]["dto.UploadJobStatusDTO"];
type WaitForUploadJobs = typeof import("./uploadLifecycle").waitForUploadJobs;

// openapi-fetch captures globalThis.fetch when the client module is evaluated,
// so the stub must be installed before uploadLifecycle is imported. The stub
// dispatches through `handler` so each test can swap behavior without needing
// a fresh module graph.
let handler: (url: string) => Promise<Response>;
let waitForUploadJobs: WaitForUploadJobs;

const jsonResponse = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });

const sseResponse = (frames: string) =>
  new Response(
    new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(frames));
        controller.close();
      },
    }),
    { status: 200, headers: { "content-type": "text/event-stream" } },
  );

const jobFrame = (event: string, job: UploadJobStatus) =>
  `event: ${event}\ndata: ${JSON.stringify({ jobs: [job] })}\n\n`;

const uploadJob = (status: string, terminal: boolean): UploadJobStatus => ({
  task_id: 42,
  file_name: "photo.jpg",
  status,
  terminal,
  success: terminal,
});

describe("upload lifecycle in a real browser", () => {
  beforeAll(async () => {
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
      return handler(url);
    });
    ({ waitForUploadJobs } = await import("./uploadLifecycle"));
  });

  afterEach(() => {
    handler = () => Promise.reject(new Error("no fetch handler installed for this test"));
  });

  afterAll(() => {
    vi.unstubAllGlobals();
  });

  it("tracks jobs over the SSE stream until the done event", async () => {
    handler = async (url) => {
      if (url.includes("/batch/jobs/stream")) {
        return sseResponse(
          jobFrame("jobs", uploadJob("running", false)) +
            jobFrame("done", uploadJob("completed", true)),
        );
      }
      return jsonResponse({}, 404);
    };

    const states: string[] = [];
    const jobs = await waitForUploadJobs([42], {
      intervalMs: 0,
      timeoutMs: 5_000,
      onUpdate: (job) => states.push(job.status ?? ""),
    });

    expect(states).toEqual(["running", "completed"]);
    expect(jobs).toHaveLength(1);
    expect(jobs[0].status).toBe("completed");
  });

  it("falls back to batch jobs polling when the SSE stream is unavailable", async () => {
    let polls = 0;
    handler = async (url) => {
      if (url.includes("/batch/jobs/stream")) return jsonResponse({}, 404);
      if (url.includes("/batch/jobs")) {
        polls += 1;
        const done = polls > 1;
        return jsonResponse({ jobs: [uploadJob(done ? "completed" : "running", done)] });
      }
      return jsonResponse({}, 404);
    };

    const states: string[] = [];
    const jobs = await waitForUploadJobs([42], {
      intervalMs: 0,
      timeoutMs: 5_000,
      onUpdate: (job) => states.push(job.status ?? ""),
    });

    expect(states).toEqual(["running", "completed"]);
    expect(jobs.at(-1)?.status).toBe("completed");
  });
});

import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vite-plus/test";
import type { components } from "@/lib/http-commons/schema.d.ts";

type UploadOperationStatus = components["schemas"]["dto.UploadOperationStatusDTO"];
type WaitForUploadOperations = typeof import("./uploadLifecycle").waitForUploadOperations;

let handler: (url: string) => Promise<Response>;
let waitForUploadOperations: WaitForUploadOperations;
const receiptId = "21a0a629-7329-4623-9f0c-a53b99878edc";

const jsonResponse = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

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

const operationFrame = (event: string, operation: UploadOperationStatus) =>
  `event: ${event}\ndata: ${JSON.stringify({ operations: [operation] })}\n\n`;

const uploadOperation = (status: string, terminal: boolean): UploadOperationStatus => ({
  receipt_id: receiptId,
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
    ({ waitForUploadOperations } = await import("./uploadLifecycle"));
  });
  afterEach(() => {
    handler = () => Promise.reject(new Error("no fetch handler installed"));
  });
  afterAll(() => {
    vi.unstubAllGlobals();
  });

  it("tracks operations over SSE until the done event", async () => {
    handler = async (url) =>
      url.includes("/batch/operations/stream")
        ? sseResponse(
            operationFrame("operations", uploadOperation("pending", false)) +
              operationFrame("done", uploadOperation("completed", true)),
          )
        : jsonResponse({}, 404);
    const states: string[] = [];
    const operations = await waitForUploadOperations([receiptId], {
      intervalMs: 0,
      timeoutMs: 5_000,
      onUpdate: (operation) => states.push(operation.status ?? ""),
    });
    expect(states).toEqual(["pending", "completed"]);
    expect(operations[0].status).toBe("completed");
  });

  it("falls back to operation polling when SSE is unavailable", async () => {
    let polls = 0;
    handler = async (url) => {
      if (url.includes("/batch/operations/stream")) return jsonResponse({}, 404);
      if (url.includes("/batch/operations")) {
        polls += 1;
        const done = polls > 1;
        return jsonResponse({
          operations: [uploadOperation(done ? "completed" : "pending", done)],
        });
      }
      return jsonResponse({}, 404);
    };
    const operations = await waitForUploadOperations([receiptId], {
      intervalMs: 0,
      timeoutMs: 5_000,
    });
    expect(operations.at(-1)?.status).toBe("completed");
  });
});

import { afterEach, describe, expect, it, vi } from "vite-plus/test";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  client: {
    GET: mocks.get,
  },
}));

import { waitForRepositoryScan } from "./waitForRepositoryScan";

afterEach(() => {
  vi.clearAllMocks();
});

describe("waitForRepositoryScan", () => {
  it("ignores an older terminal run and resolves the requested completed run", async () => {
    const requestedAt = Date.parse("2026-07-16T12:00:00.000Z");
    mocks.get
      .mockResolvedValueOnce({
        data: {
          started_at: "2026-07-16T11:59:30.000Z",
          status: "completed",
        },
        response: { status: 200 },
      })
      .mockResolvedValueOnce({
        data: {
          started_at: "2026-07-16T12:00:00.000Z",
          status: "completed",
        },
        response: { status: 200 },
      });

    await expect(
      waitForRepositoryScan("repository-1", requestedAt, {
        intervalMs: 0,
        timeoutMs: 1_000,
      }),
    ).resolves.toMatchObject({ status: "completed" });
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("rejects a requested terminal failure with the backend error", async () => {
    const requestedAt = Date.parse("2026-07-16T12:00:00.000Z");
    mocks.get.mockResolvedValue({
      data: {
        started_at: "2026-07-16T12:00:00.000Z",
        status: "failed",
        error: "scan failed",
      },
      response: { status: 200 },
    });

    await expect(
      waitForRepositoryScan("repository-1", requestedAt, {
        intervalMs: 0,
        timeoutMs: 1_000,
      }),
    ).rejects.toThrow("scan failed");
  });

  it("rejects non-404 status errors immediately", async () => {
    mocks.get.mockResolvedValue({
      error: { message: "service unavailable" },
      response: { status: 503 },
    });

    await expect(
      waitForRepositoryScan("repository-1", Date.now(), {
        intervalMs: 0,
        timeoutMs: 1_000,
      }),
    ).rejects.toThrow("service unavailable");
  });
});

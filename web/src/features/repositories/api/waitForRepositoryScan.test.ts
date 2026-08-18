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

  it("rejects a requested terminal failure with its Problem Reference", async () => {
    const requestedAt = Date.parse("2026-07-16T12:00:00.000Z");
    const problem = {
      type: "https://lumilio.org/problems/repository/scan-failed",
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      retryable: true,
    };
    mocks.get.mockResolvedValue({
      data: {
        started_at: "2026-07-16T12:00:00.000Z",
        status: "failed",
        problem,
      },
      response: { status: 200 },
    });

    await expect(
      waitForRepositoryScan("repository-1", requestedAt, {
        intervalMs: 0,
        timeoutMs: 1_000,
      }),
    ).rejects.toEqual(problem);
  });

  it("rejects non-404 request failures as normalized Problems", async () => {
    const error = {
      type: "https://lumilio.org/problems/service/unavailable",
      status: 503,
      instance: "urn:lumilio:problem:fedcba9876543210fedcba9876543210",
    };
    mocks.get.mockResolvedValue({
      error,
      response: { status: 503 },
    });

    await expect(
      waitForRepositoryScan("repository-1", Date.now(), {
        intervalMs: 0,
        timeoutMs: 1_000,
      }),
    ).rejects.toEqual({
      kind: "problem",
      ...error,
      retryAfterSeconds: undefined,
      conflictType: undefined,
      repositoryID: undefined,
      actions: undefined,
    });
  });
});

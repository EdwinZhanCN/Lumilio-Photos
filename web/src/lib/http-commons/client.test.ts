import { beforeEach, describe, expect, it, vi } from "vitest";

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

describe("authenticated OpenAPI client", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it("serializes refresh-token rotation across concurrent 401 responses", async () => {
    localStorage.setItem("auth_token", "expired-access");
    localStorage.setItem("refresh_token", "refresh-one");

    let initialRequests = 0;
    let refreshRequests = 0;
    let releaseRefresh: (() => void) | undefined;
    const bothRequestsStarted = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });

    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const request = input instanceof Request ? input : new Request(input, init);
      if (request.url.endsWith("/api/v1/auth/refresh")) {
        refreshRequests += 1;
        await bothRequestsStarted;
        return json({ token: "fresh-access", refreshToken: "refresh-two" });
      }

      if (request.headers.get("Authorization") === "Bearer expired-access") {
        initialRequests += 1;
        if (initialRequests === 2) releaseRefresh?.();
        return json({ message: "expired" }, 401);
      }

      return json({ user_id: 1, username: "alice" });
    });
    vi.stubGlobal("fetch", fetchMock);

    const { client } = await import("./client.ts");
    const [first, second] = await Promise.all([
      client.GET("/api/v1/auth/me"),
      client.GET("/api/v1/auth/me"),
    ]);

    expect(first.response.status).toBe(200);
    expect(second.response.status).toBe(200);
    expect(refreshRequests).toBe(1);
    expect(localStorage.getItem("auth_token")).toBe("fresh-access");
    expect(localStorage.getItem("refresh_token")).toBe("refresh-two");
  });

  it("replays a mutation from a pristine request-body clone", async () => {
    localStorage.setItem("auth_token", "expired-access");
    localStorage.setItem("refresh_token", "refresh-one");

    const seenBodies: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const request = input instanceof Request ? input : new Request(input, init);
      if (request.url.endsWith("/api/v1/auth/refresh")) {
        return json({ token: "fresh-access", refreshToken: "refresh-two" });
      }

      seenBodies.push(await request.text());
      if (request.headers.get("Authorization") === "Bearer expired-access") {
        return json({ message: "expired" }, 401);
      }
      return json({ results: [] });
    });
    vi.stubGlobal("fetch", fetchMock);

    const { client } = await import("./client.ts");
    const result = await client.POST("/api/v1/assets/precheck", {
      body: { files: [{ hash: "abc", size: 123 }] },
    });

    expect(result.response.status).toBe(200);
    expect(seenBodies).toEqual([
      JSON.stringify({ files: [{ hash: "abc", size: 123 }] }),
      JSON.stringify({ files: [{ hash: "abc", size: 123 }] }),
    ]);
  });
});

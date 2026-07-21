import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { client } from "./client.ts";
import { registerSessionExpiredHandler } from "./sessionEvents.ts";

// The real OpenAPI client runs against MSW at the HTTP boundary, so the
// 401 → refresh → retry rotation, body replay and session-expiry signalling are
// exercised end to end without stubbing fetch (which fights the MSW worker).

describe("authenticated OpenAPI client", () => {
  beforeEach(() => {
    localStorage.clear();
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

    worker.use(
      http.post("*/api/v1/auth/refresh", async () => {
        refreshRequests += 1;
        await bothRequestsStarted;
        return HttpResponse.json({ token: "fresh-access", refreshToken: "refresh-two" });
      }),
      http.get("*/api/v1/auth/me", ({ request }) => {
        if (request.headers.get("Authorization") === "Bearer expired-access") {
          initialRequests += 1;
          if (initialRequests === 2) releaseRefresh?.();
          return HttpResponse.json({ message: "expired" }, { status: 401 });
        }
        return HttpResponse.json({ user_id: 1, username: "alice" });
      }),
    );

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
    worker.use(
      http.post("*/api/v1/auth/refresh", () =>
        HttpResponse.json({ token: "fresh-access", refreshToken: "refresh-two" }),
      ),
      http.post("*/api/v1/assets/precheck", async ({ request }) => {
        seenBodies.push(await request.text());
        if (request.headers.get("Authorization") === "Bearer expired-access") {
          return HttpResponse.json({ message: "expired" }, { status: 401 });
        }
        return HttpResponse.json({ results: [] });
      }),
    );

    const result = await client.POST("/api/v1/assets/precheck", {
      body: { files: [{ hash: "abc", size: 123 }] },
    });

    expect(result.response.status).toBe(200);
    expect(seenBodies).toEqual([
      JSON.stringify({ files: [{ hash: "abc", size: 123 }] }),
      JSON.stringify({ files: [{ hash: "abc", size: 123 }] }),
    ]);
  });

  it("notifies the session owner only once when refresh cannot recover", async () => {
    localStorage.setItem("auth_token", "expired-access");
    localStorage.setItem("refresh_token", "expired-refresh");

    worker.use(
      http.post("*/api/v1/auth/refresh", () =>
        HttpResponse.json({ message: "invalid refresh token" }, { status: 401 }),
      ),
      http.get("*/api/v1/auth/me", () =>
        HttpResponse.json({ message: "expired access token" }, { status: 401 }),
      ),
    );

    const handleSessionExpired = vi.fn();
    const unregister = registerSessionExpiredHandler(handleSessionExpired);

    await client.GET("/api/v1/auth/me");
    await client.GET("/api/v1/auth/me");

    expect(handleSessionExpired).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem("auth_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    unregister();
  });
});

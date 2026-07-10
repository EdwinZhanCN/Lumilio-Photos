/**
 * openapi-fetch client with authentication middleware
 *
 * This client provides type-safe API requests using the generated OpenAPI schema.
 * It handles JWT token management and automatic token refresh.
 */
import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";
import { getToken, getRefreshToken, saveToken, removeToken } from "./auth.ts";
import { notifySessionExpired } from "@/features/auth/sessionEvents.ts";

export const baseUrl = import.meta.env.VITE_API_URL ?? "";

let refreshInFlight: Promise<string | null> | null = null;
let refreshGeneration = 0;
let refreshAbortController: AbortController | null = null;
const replayRequests = new Map<string, Request>();

const isRefreshPayload = (
  value: unknown,
): value is { token: string; refreshToken: string } => {
  if (!value || typeof value !== "object") return false;
  return (
    "token" in value &&
    typeof value.token === "string" &&
    value.token.length > 0 &&
    "refreshToken" in value &&
    typeof value.refreshToken === "string" &&
    value.refreshToken.length > 0
  );
};

async function refreshAccessToken(fetcher: typeof fetch): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  const refreshToken = getRefreshToken();
  if (!refreshToken) return null;

  const generation = refreshGeneration;
  const controller = new AbortController();
  refreshAbortController = controller;

  const pending = fetcher(`${baseUrl}/api/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refreshToken }),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) return null;
      const payload: unknown = await response.json();
      if (!isRefreshPayload(payload)) return null;
      if (generation !== refreshGeneration || getRefreshToken() !== refreshToken) return null;

      saveToken(payload.token, payload.refreshToken);
      return payload.token;
    })
    .catch((error: unknown) => {
      if (error instanceof DOMException && error.name === "AbortError") return null;
      return null;
    })
    .finally(() => {
      if (refreshInFlight === pending) refreshInFlight = null;
      if (refreshAbortController === controller) refreshAbortController = null;
    });

  refreshInFlight = pending;
  return pending;
}

/** Prevent a late refresh response from recreating a session after logout. */
export function invalidateAuthRefresh(): void {
  refreshGeneration += 1;
  refreshAbortController?.abort();
  refreshAbortController = null;
  refreshInFlight = null;
  replayRequests.clear();
}

/** Auth middleware adds the access token, serializes rotation, and replays once. */
export const authMiddleware: Middleware = {
  async onRequest({ request, id }) {
    const token = getToken();
    if (token) {
      request.headers.set("Authorization", `Bearer ${token}`);
    }
    replayRequests.set(id, request.clone());
    return request;
  },
  async onResponse({ response, request, id, options }) {
    const replay = replayRequests.get(id);
    replayRequests.delete(id);
    if (response.status !== 401) return response;

    const url = request.url;
    if (
      url.includes("/auth/refresh") ||
      url.includes("/auth/login") ||
      url.includes("/auth/register") ||
      url.includes("/auth/passkeys/login") ||
      url.includes("/auth/mfa/verify")
    ) {
      return response;
    }

    const currentToken = getToken();
    const requestAuthorization = replay?.headers.get("Authorization");
    const token =
      currentToken && requestAuthorization !== `Bearer ${currentToken}`
        ? currentToken
        : await refreshAccessToken(options.fetch);
    if (!token || !replay) {
      invalidateAuthRefresh();
      removeToken();
      notifySessionExpired();
      return response;
    }

    const headers = new Headers(replay.headers);
    headers.set("Authorization", `Bearer ${token}`);
    return options.fetch(new Request(replay, { headers }));
  },
  onError({ id }) {
    replayRequests.delete(id);
  },
};

/**
 * Typed openapi-fetch client
 *
 * Usage:
 * ```ts
 * const { data, error } = await client.GET("/api/v1/health");
 * ```
 */
export const client = createClient<paths>({ baseUrl });
client.use(authMiddleware);

export default client;

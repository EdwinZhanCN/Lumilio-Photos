/**
 * openapi-fetch client with authentication middleware
 *
 * This client provides type-safe API requests using the generated OpenAPI schema.
 * It handles JWT token management and automatic token refresh.
 */
import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";
import { getCSRFToken, getToken, removeToken, saveCSRFToken, saveToken } from "./auth.ts";
import { notifySessionExpired } from "./sessionEvents.ts";

export const baseUrl = import.meta.env.VITE_API_URL ?? "";

let refreshInFlight: Promise<string | null> | null = null;
let refreshGeneration = 0;
let refreshAbortController: AbortController | null = null;
const replayRequests = new Map<string, Request>();

const isRefreshPayload = (value: unknown): value is { token: string; csrfToken: string } => {
  if (!value || typeof value !== "object") return false;
  return (
    "token" in value &&
    typeof value.token === "string" &&
    value.token.length > 0 &&
    "csrfToken" in value &&
    typeof value.csrfToken === "string" &&
    value.csrfToken.length > 0
  );
};

const isCSRFPayload = (value: unknown): value is { csrfToken: string } =>
  Boolean(
    value &&
    typeof value === "object" &&
    "csrfToken" in value &&
    typeof value.csrfToken === "string" &&
    value.csrfToken.length > 0,
  );

async function fetchSessionCSRFToken(
  fetcher: typeof fetch,
  signal: AbortSignal,
): Promise<string | null> {
  try {
    const response = await fetcher(`${baseUrl}/api/v1/auth/csrf`, {
      method: "GET",
      credentials: "include",
      signal,
    });
    if (!response.ok) return null;
    const payload: unknown = await response.json();
    if (!isCSRFPayload(payload)) return null;
    saveCSRFToken(payload.csrfToken);
    return payload.csrfToken;
  } catch (error: unknown) {
    if (error instanceof DOMException && error.name === "AbortError") return null;
    return null;
  }
}

async function withBrowserSessionLock<T>(operation: () => Promise<T>): Promise<T> {
  if (typeof navigator !== "undefined" && navigator.locks) {
    return navigator.locks.request("lumilio-auth-refresh", { mode: "exclusive" }, operation);
  }
  return operation();
}

async function refreshAccessToken(fetcher: typeof fetch): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  const generation = refreshGeneration;
  const accessTokenAtStart = getToken();
  const controller = new AbortController();
  refreshAbortController = controller;

  const pending = withBrowserSessionLock(async () => {
    if (generation !== refreshGeneration) return null;
    const currentAccessToken = getToken();
    if (currentAccessToken && currentAccessToken !== accessTokenAtStart) {
      return currentAccessToken;
    }

    const csrfToken = await fetchSessionCSRFToken(fetcher, controller.signal);
    if (!csrfToken || generation !== refreshGeneration) return null;

    try {
      const response = await fetcher(`${baseUrl}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
        headers: { "X-CSRF-Token": csrfToken },
        signal: controller.signal,
      });
      if (!response.ok) return null;
      const payload: unknown = await response.json();
      if (!isRefreshPayload(payload)) return null;
      if (generation !== refreshGeneration) return null;

      saveToken(payload.token, payload.csrfToken);
      return payload.token;
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") return null;
      return null;
    }
  }).finally(() => {
    if (refreshInFlight === pending) refreshInFlight = null;
    if (refreshAbortController === controller) refreshAbortController = null;
  });

  refreshInFlight = pending;
  return pending;
}

/** Restore/rotate the HttpOnly refresh-cookie session during app bootstrap. */
export function refreshBrowserSession(): Promise<string | null> {
  const browserFetch = globalThis.fetch.bind(globalThis);
  return refreshAccessToken(browserFetch);
}

/** Revoke the cookie session under the same cross-tab lock used for rotation. */
export function logoutBrowserSession(): Promise<boolean> {
  const browserFetch = globalThis.fetch.bind(globalThis);
  return withBrowserSessionLock(async () => {
    if (refreshInFlight) await refreshInFlight;
    const controller = new AbortController();
    const csrfToken = await fetchSessionCSRFToken(browserFetch, controller.signal);
    if (!csrfToken) return false;
    try {
      const response = await browserFetch(`${baseUrl}/api/v1/auth/logout`, {
        method: "POST",
        credentials: "include",
        headers: { "X-CSRF-Token": csrfToken },
        signal: controller.signal,
      });
      return response.ok;
    } catch {
      return false;
    }
  });
}

/** Prevent a late refresh response from recreating a session after logout. */
export function invalidateAuthRefresh(): void {
  refreshGeneration += 1;
  refreshAbortController?.abort();
  refreshAbortController = null;
  refreshInFlight = null;
  replayRequests.clear();
}

/**
 * Authenticated fetch for annotated endpoints that are newer than the checked-in
 * generated TypeScript client. It uses the same token rotation and CSRF policy
 * as openapi-fetch, so feature code never recreates authentication semantics.
 * `task dto` remains the source-of-truth update that moves these calls back to
 * the typed client after OpenAPI regeneration.
 */
export async function authenticatedFetch(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const browserFetch = globalThis.fetch.bind(globalThis);
  const request = new Request(`${baseUrl}${path}`, {
    ...init,
    credentials: "include",
  });
  const attachSession = (source: Request, token = getToken()) => {
    const headers = new Headers(source.headers);
    if (token) headers.set("Authorization", `Bearer ${token}`);
    const csrfToken = getCSRFToken();
    if (csrfToken && !["GET", "HEAD", "OPTIONS"].includes(source.method.toUpperCase())) {
      headers.set("X-CSRF-Token", csrfToken);
    }
    return new Request(source, { headers });
  };

  const replay = request.clone();
  let response = await browserFetch(attachSession(request));
  if (response.status !== 401) return response;

  const token = await refreshAccessToken(browserFetch);
  if (!token) {
    invalidateAuthRefresh();
    removeToken();
    notifySessionExpired();
    return response;
  }
  response = await browserFetch(attachSession(replay, token));
  return response;
}

/** Auth middleware adds the access token, serializes rotation, and replays once. */
export const authMiddleware: Middleware = {
  async onRequest({ request, id }) {
    const token = getToken();
    if (token) {
      request.headers.set("Authorization", `Bearer ${token}`);
    }
    const csrfToken = getCSRFToken();
    if (csrfToken && !["GET", "HEAD", "OPTIONS"].includes(request.method.toUpperCase())) {
      request.headers.set("X-CSRF-Token", csrfToken);
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
      url.includes("/auth/csrf") ||
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
    // Call through a local binding: `options.fetch(...)` invokes native fetch with
    // `this === options`, which throws "Illegal invocation" in the browser.
    const { fetch: forwardRequest } = options;
    return forwardRequest(new Request(replay, { headers }));
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
export const client = createClient<paths>({ baseUrl, credentials: "include" });
client.use(authMiddleware);

export default client;

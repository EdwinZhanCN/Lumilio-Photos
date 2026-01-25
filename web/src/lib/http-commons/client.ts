/**
 * openapi-fetch client with authentication middleware
 * 
 * This client provides type-safe API requests using the generated OpenAPI schema.
 * It handles JWT token management and automatic token refresh.
 */
import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";
import { getToken, getRefreshToken, saveToken, removeToken } from "./api";

export const baseUrl = import.meta.env.VITE_API_URL || "http://localhost:8080";

/**
 * Auth middleware - adds Bearer token to requests and handles 401 refresh
 */
const authMiddleware: Middleware = {
  async onRequest({ request }) {
    const token = getToken();
    if (token) {
      request.headers.set("Authorization", `Bearer ${token}`);
    }
    return request;
  },
  async onResponse({ response, request }) {
    // Handle 401 Unauthorized - attempt token refresh
    if (response.status === 401) {
      const url = request.url;
      // Don't attempt refresh for auth endpoints to avoid loops
      if (url.includes("/auth/refresh") || url.includes("/auth/login")) {
        return response;
      }

      try {
        const refreshToken = getRefreshToken();
        if (refreshToken) {
          const refreshResponse = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ refreshToken }),
          });

          if (refreshResponse.ok) {
            const data = await refreshResponse.json();
            if (data.code === 0 && data.data) {
              const { token, refreshToken: newRefreshToken } = data.data;
              saveToken(token, newRefreshToken);

              // Retry original request with new token
              const newRequest = new Request(request.url, {
                method: request.method,
                headers: new Headers(request.headers),
                body: request.body,
              });
              newRequest.headers.set("Authorization", `Bearer ${token}`);
              return fetch(newRequest);
            }
          }
        }
      } catch {
        removeToken();
        window.location.href = "/login";
      }
    }
    return response;
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
import { http, HttpResponse, worker } from "./msw";

/**
 * Seed a real authenticated session for flow specs: store tokens and answer the
 * auth bootstrap (`/auth/me` + media token) so the real AuthProvider settles to
 * `user` instead of idle. Pair with `renderWithProviders(ui, { auth: true })`.
 * Storage is cleared between tests by the integration setup.
 */
export function seedSession(user: Record<string, unknown>) {
  localStorage.setItem("auth_token", "test-access");
  localStorage.setItem("refresh_token", "test-refresh");
  worker.use(
    http.get("*/api/v1/auth/me", () => HttpResponse.json(user)),
    http.get("*/api/v1/auth/media-token", () =>
      HttpResponse.json({
        token: "media",
        expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      }),
    ),
  );
}

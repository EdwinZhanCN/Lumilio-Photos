import { http, HttpResponse } from "msw";
import { setupWorker } from "msw/browser";

// Shared MSW worker for the integration project. Specs declare their business
// responses per test with `worker.use(...)`; every AuthProvider performs one
// HttpOnly-cookie probe, so unauthenticated specs get the real 401 shape by
// default and authenticated specs override it through seedSession.
export const worker = setupWorker(
  http.get("*/api/v1/auth/csrf", () =>
    HttpResponse.json({ message: "no refresh-cookie session" }, { status: 401 }),
  ),
);

export { http, HttpResponse };

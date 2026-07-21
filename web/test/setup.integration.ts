import { afterAll, afterEach, beforeAll, vi } from "vite-plus/test";
import { worker } from "./msw";

// Integration tests run in real Chromium, so localStorage, CSS and browser APIs
// are the real implementations — no polyfills. vitest-browser-react cleans up
// rendered components on its own; this only resets state that persists within a
// browser context across tests in the same file, and the MSW handlers.

beforeAll(async () => {
  await worker.start({
    quiet: true,
    // Guard the API surface only: Vite still has to serve modules and assets, so
    // erroring on every unhandled request would break the page itself.
    onUnhandledRequest(request, print) {
      if (new URL(request.url).pathname.startsWith("/api/")) print.error();
    },
  });
});

afterEach(() => {
  worker.resetHandlers();
  localStorage.clear();
  sessionStorage.clear();
  vi.clearAllMocks();
});

afterAll(() => worker.stop());

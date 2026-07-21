import { setupWorker } from "msw/browser";

// Shared MSW worker for the integration project. Specs declare their business
// responses per test with `worker.use(...)`; only requests that every test makes
// regardless of subject belong in the default handlers here (currently none).
export const worker = setupWorker();

export { http, HttpResponse } from "msw";

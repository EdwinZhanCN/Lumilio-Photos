import { chromium } from "playwright";
import { preview as createPreview } from "vite";

const host = "127.0.0.1";
const requestedPort = Number(process.env.PRODUCTION_SMOKE_PORT ?? 0);

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

assert(
  Number.isInteger(requestedPort) && requestedPort >= 0 && requestedPort <= 65_535,
  `Invalid PRODUCTION_SMOKE_PORT: ${process.env.PRODUCTION_SMOKE_PORT}`,
);

let previewServer;
let origin;

const startPreview = async () => {
  previewServer = await createPreview({
    preview: {
      host,
      port: requestedPort,
      strictPort: requestedPort !== 0,
    },
  });

  const previewUrl = previewServer.resolvedUrls?.local[0];
  assert(previewUrl, "Production preview started without a local URL");
  origin = new URL(previewUrl).origin;

  const deadline = Date.now() + 20_000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${origin}/production-smoke.html`);
      const body = await response.text();
      if (response.ok && body.includes("<title>Lumilio production smoke</title>")) return;
      lastError = new Error(
        response.ok
          ? "production-smoke.html resolved to the SPA fallback"
          : `HTTP ${response.status}`,
      );
    } catch (error) {
      // Preview is still starting.
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(
    `Timed out waiting for production preview at ${origin}: ${lastError?.message ?? "unknown error"}`,
  );
};

let browser;
try {
  await startPreview();
  browser = await chromium.launch({
    channel: process.env.PLAYWRIGHT_CHANNEL ?? "chrome",
    headless: true,
  });
  const page = await browser.newPage();
  await page.goto(`${origin}/production-smoke.html`);
  await page.waitForFunction(() => Boolean(window.__lumilioProductionSmoke));

  assert(await page.evaluate(() => globalThis.crossOriginIsolated), "crossOriginIsolated is false");
  const digest = await page.evaluate(() => window.__lumilioProductionSmoke.hash());
  assert(/^[a-f0-9]{64}$/.test(digest), "BLAKE3 worker returned an invalid digest");

  await page.route("**/api/v1/assets", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: '{"message":"storage unavailable"}',
    }),
  );
  const failure = await page.evaluate(() => window.__lumilioProductionSmoke.uploadFailure());
  assert(failure.includes("503"), "non-2xx upload response was accepted");
  await page.unroute("**/api/v1/assets");

  let uploadPoll = 0;
  await page.route("**/api/v1/assets/batch/jobs**", (route) => {
    if (new URL(route.request().url()).pathname.endsWith("/stream")) {
      return route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
    }
    uploadPoll += 1;
    const complete = uploadPoll > 1;
    return route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        jobs: [
          {
            task_id: 42,
            file_name: "photo.jpg",
            status: complete ? "completed" : "running",
            terminal: complete,
            success: complete,
          },
        ],
      }),
    });
  });
  const uploadStates = await page.evaluate(() => window.__lumilioProductionSmoke.uploadLifecycle());
  assert(
    uploadStates.join(",") === "running,completed",
    "upload lifecycle did not reach completion",
  );

  let scanPoll = 0;
  await page.route("**/api/v1/repositories/repo-1/scans/latest", (route) => {
    scanPoll += 1;
    return route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        scan_id: "scan-1",
        repository_id: "repo-1",
        status: scanPoll > 1 ? "completed" : "running",
        started_at: new Date().toISOString(),
      }),
    });
  });
  const scanStatus = await page.evaluate(() => window.__lumilioProductionSmoke.scanLifecycle());
  assert(scanStatus === "completed", "repository scan lifecycle did not reach completion");

  process.stdout.write(
    "Production browser smoke passed: isolation, BLAKE3, upload recovery, and lifecycle transitions.\n",
  );
} finally {
  await browser?.close();
  await previewServer?.close();
}

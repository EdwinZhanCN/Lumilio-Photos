import { defineConfig } from "playwright/test";

export default defineConfig({
  testDir: "./e2e/specs",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [["github"], ["html", { outputFolder: "playwright-report", open: "never" }]]
    : "list",
  outputDir: "test-results",
  use: {
    baseURL: process.env.LUMILIO_E2E_BASE_URL ?? "http://127.0.0.1:16657",
    browserName: "chromium",
    headless: true,
    // Pins the language i18next detects from `navigator`, so accessible names
    // resolve from the `en` bundle that `e2e/support/i18n.ts` reads.
    locale: "en-US",
    testIdAttribute: "data-testid",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
});

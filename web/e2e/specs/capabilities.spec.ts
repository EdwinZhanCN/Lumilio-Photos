import { expect, test } from "../fixtures/test";

test("@smoke app is served cross-origin isolated for workers and WASM", async ({ page }) => {
  await page.goto("/login");
  expect(await page.evaluate(() => globalThis.crossOriginIsolated)).toBe(true);
});

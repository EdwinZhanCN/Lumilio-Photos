import { test as base, expect } from "playwright/test";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

export const credentials = {
  username: process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin",
  password: process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!",
};

/** The fields specs consume; `seed.json` records the full seeded state. */
type SeedState = {
  scanFilename: string;
  uploadAsset: string;
};

/** State written by `e2e/support/seed.mjs`. Data anchors, not UI copy. */
export const seed: SeedState = JSON.parse(
  readFileSync(path.join(repositoryRoot, ".cache/e2e/seed.json"), "utf8"),
);

export const smokeAsset = (relativePath: string) => {
  const lock = JSON.parse(readFileSync(path.join(repositoryRoot, "assets.lock.json"), "utf8"));
  return path.join(repositoryRoot, ".cache/lumilio-assets", lock.revision, "smoke", relativePath);
};

export const test = base;
export { expect };

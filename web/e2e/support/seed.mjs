import { readFile, writeFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const baseURL = process.env.LUMILIO_E2E_BASE_URL ?? "http://127.0.0.1:16657";
const username = process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin";
const password = process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!";

async function request(pathname, init = {}) {
  const response = await fetch(`${baseURL}${pathname}`, {
    ...init,
    headers: { "content-type": "application/json", ...init.headers },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new Error(
      `${init.method ?? "GET"} ${pathname}: ${response.status} ${JSON.stringify(body)}`,
    );
  return body;
}

const status = await request("/api/v1/setup/status");
if (!status.database_initialized) await request("/api/v1/setup", { method: "POST", body: "{}" });

let auth;
if (!status.admin_initialized) {
  auth = await request("/api/v1/auth/register/start", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
} else {
  auth = await request("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}
const headers = { authorization: `Bearer ${auth.token}` };
const repositories = await request("/api/v1/repositories", { headers }).catch(() => ({
  repositories: [],
}));
let primary = repositories.repositories?.find((repository) => repository.is_primary);
if (!primary) {
  const created = await request("/api/v1/repositories", {
    method: "POST",
    headers,
    body: JSON.stringify({
      name: "E2E Primary",
      role: "primary",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  primary = created.repository;
}

const lock = JSON.parse(await readFile(path.join(root, "assets.lock.json"), "utf8"));
const assetRoot = path.join(root, ".cache/lumilio-assets", lock.revision, "smoke");
const catalog = JSON.parse(await readFile(path.join(assetRoot, "assets.json"), "utf8"));
const scanAsset = catalog.assets.find((asset) => asset.id === "picsum-scan-000");
if (!scanAsset) throw new Error("smoke profile is missing picsum-scan-000");
// Storage is a named volume, so the fixture goes in through the container
// rather than the host filesystem.
const scanFilename = "e2e-scan-000.jpg";
const scanTarget = `/data/storage/primary/${scanFilename}`;
const copy = spawnSync(
  "docker",
  [
    "compose",
    "-f",
    "docker-compose.e2e.yml",
    "-p",
    "lumilio-photos-e2e",
    "cp",
    path.join(assetRoot, scanAsset.path),
    `server:${scanTarget}`,
  ],
  { cwd: root, stdio: "inherit" },
);
if (copy.error) throw copy.error;
if (copy.status !== 0) throw new Error(`docker compose cp failed (${copy.status})`);

// Specs read this instead of restating seeded names, so they assert against the
// state that actually exists rather than a copy that can drift.
const state = {
  username,
  repositoryId: primary.id,
  repositoryName: primary.name,
  scanFilename,
  uploadAsset: "media/upload/upload-001.jpg",
};
await writeFile(path.join(root, ".cache/e2e/seed.json"), JSON.stringify(state, null, 2));

console.log(JSON.stringify({ ...state, scanFile: scanTarget }));

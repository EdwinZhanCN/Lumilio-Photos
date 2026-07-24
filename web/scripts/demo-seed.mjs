// Materializes the pinned `demo` asset profile into a running Lumilio instance
// through the real setup, repository and upload APIs. Shares profile resolution
// and verification with the E2E seed via assets-sync.mjs; the only difference is
// that this profile selects the full demonstration pool instead of the minimal
// smoke subset.
import { readFile } from "node:fs/promises";
import { openAsBlob } from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { parseLock, selectProfile, syncAssets } from "./assets-sync.mjs";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = path.resolve(webRoot, "..");

const baseURL = process.env.LUMILIO_DEMO_BASE_URL ?? "http://localhost:6680";
const username = process.env.LUMILIO_DEMO_USERNAME ?? "lumilio-demo";
const password = process.env.LUMILIO_DEMO_PASSWORD ?? "Lumilio-Demo-2026!";
const repositoryName = process.env.LUMILIO_DEMO_REPOSITORY ?? "Lumilio Demo";

function parseOptions(args) {
  const options = { concurrency: 4, timeoutMs: 20 * 60 * 1000 };
  for (let index = 0; index < args.length; index += 1) {
    const [flag, inline] = args[index].split("=");
    const value = inline ?? args[++index];
    if (flag === "--concurrency") options.concurrency = Number(value);
    else if (flag === "--timeout") options.timeoutMs = Number(value) * 1000;
    else throw new Error(`unknown argument: ${args[index]}`);
  }
  if (!Number.isInteger(options.concurrency) || options.concurrency < 1 || options.concurrency > 8) {
    throw new Error("--concurrency must be between 1 and 8");
  }
  if (!Number.isFinite(options.timeoutMs) || options.timeoutMs <= 0) {
    throw new Error("--timeout must be a positive number of seconds");
  }
  return options;
}

async function api(pathname, { method = "GET", body, token, form } = {}) {
  const response = await fetch(`${baseURL}${pathname}`, {
    method,
    body: form ?? body,
    headers: {
      ...(form ? {} : { "content-type": "application/json" }),
      ...(token ? { authorization: `Bearer ${token}` } : {}),
    },
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`${method} ${pathname}: ${response.status} ${JSON.stringify(payload)}`);
  }
  return payload;
}

/** Brings an empty instance up to an authenticated admin session. */
async function ensureAdmin() {
  const status = await api("/api/v1/setup/status");
  if (!status.database_initialized) await api("/api/v1/setup", { method: "POST", body: "{}" });

  if (!status.admin_initialized) {
    return api("/api/v1/auth/register/start", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  }
  return api("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

/**
 * Demo media lands in its own named repository so it never mixes with a
 * developer's real library. An existing repository is reused, never recreated.
 *
 * A fresh instance is not "app initialized" until a primary repository exists,
 * and `GET /repositories` is gated behind that readiness — so on first run we
 * must create the primary repository through the ungated `POST /repositories`
 * before any gated call. On a fresh instance the demo repository itself becomes
 * the primary; when a primary already exists (e.g. a developer's real library),
 * the demo lands in a separate regular repository.
 */
async function ensureRepository(token) {
  const status = await api("/api/v1/setup/status");

  if (!status.primary_repository_initialized) {
    try {
      const { repository } = await api("/api/v1/repositories", {
        method: "POST",
        token,
        body: JSON.stringify({
          name: repositoryName,
          role: "primary",
          storage_strategy: "date",
          duplicate_handling: "rename",
        }),
      });
      return repository;
    } catch (error) {
      // A primary created concurrently or by a prior partial run is fine; fall
      // through to discover and reuse it below.
      if (!String(error).includes("primary_exists")) throw error;
    }
  }

  const { repositories } = await api("/api/v1/repositories", { token });
  const existing = repositories?.find((candidate) => candidate.name === repositoryName);
  if (existing) return existing;

  const { repository } = await api("/api/v1/repositories", {
    method: "POST",
    token,
    body: JSON.stringify({
      name: repositoryName,
      role: "regular",
      storage_strategy: "date",
      duplicate_handling: "rename",
    }),
  });
  return repository;
}

async function countAssets(token, repositoryId) {
  // QueryAssetsResponseDTO.total_assets counts assets rather than browse items,
  // so stacking does not change the number.
  const payload = await api("/api/v1/assets/list", {
    method: "POST",
    token,
    body: JSON.stringify({
      filter: { repository_id: repositoryId },
      pagination: { limit: 1, offset: 0 },
    }),
  });
  return payload.total_assets ?? 0;
}

async function uploadAll(assets, token, repositoryId, concurrency) {
  let next = 0;
  let done = 0;
  const failures = [];

  async function worker() {
    while (next < assets.length) {
      const asset = assets[next++];
      const form = new FormData();
      form.append("file", await openAsBlob(asset.absolutePath), path.basename(asset.path));
      form.append("repository_id", repositoryId);
      try {
        await api("/api/v1/assets", { method: "POST", token, form });
      } catch (error) {
        failures.push(`${asset.id}: ${error instanceof Error ? error.message : error}`);
      }
      done += 1;
      if (done % 25 === 0 || done === assets.length) {
        console.log(`  uploaded ${done}/${assets.length}`);
      }
    }
  }

  await Promise.all(Array.from({ length: concurrency }, worker));
  return failures;
}

/** Ingestion continues after upload responses, so wait for the count to settle. */
async function waitForIngestion(token, repositoryId, expected, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let last = -1;
  while (Date.now() < deadline) {
    const total = await countAssets(token, repositoryId);
    if (total >= expected) return total;
    if (total !== last) {
      console.log(`  ingested ${total}/${expected}`);
      last = total;
    }
    await new Promise((resolve) => setTimeout(resolve, 2000));
  }
  throw new Error(`ingestion did not reach ${expected} assets before the timeout`);
}

async function main() {
  const options = parseOptions(process.argv.slice(2));
  const lockPath = path.join(repositoryRoot, "assets.lock.json");
  const lock = parseLock(await readFile(lockPath, "utf8"));

  const { target, cached } = await syncAssets({
    lockPath,
    cacheRoot: path.join(repositoryRoot, ".cache/lumilio-assets"),
    profileName: "demo",
  });
  console.log(`${cached ? "Verified cached" : "Synchronized"} demo assets at ${target}`);

  const catalog = JSON.parse(await readFile(path.join(target, "assets.json"), "utf8"));
  const profile = JSON.parse(await readFile(path.join(target, "profiles/demo.json"), "utf8"));
  const assets = selectProfile(catalog, profile, "demo").map((asset) => ({
    ...asset,
    absolutePath: path.join(target, asset.path),
  }));

  const { token } = await ensureAdmin();
  const repository = await ensureRepository(token);
  const before = await countAssets(token, repository.id);
  console.log(`Repository ${repository.name} (${repository.id}) holds ${before} assets`);

  if (before >= assets.length) {
    console.log(`Nothing to do: ${assets.length} demo assets are already present`);
    return;
  }

  console.log(`Uploading ${assets.length} assets at concurrency ${options.concurrency}`);
  const failures = await uploadAll(assets, token, repository.id, options.concurrency);
  if (failures.length > 0) {
    throw new Error(`${failures.length} uploads failed:\n  ${failures.slice(0, 5).join("\n  ")}`);
  }

  const total = await waitForIngestion(token, repository.id, assets.length, options.timeoutMs);
  console.log(`Demo library ready: ${total} assets in ${repository.name}`);
  console.log(`Sign in as ${username} at ${baseURL}`);
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});

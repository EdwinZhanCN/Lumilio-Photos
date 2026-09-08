// Materializes the pinned `demo` asset profile into a running Lumilio instance
// through the real setup, repository and upload APIs. Shares profile resolution
// and verification with the E2E seed via assets-sync.ts; the only difference is
// that this profile selects the full demonstration pool instead of the minimal
// smoke subset.
import { readFile } from "node:fs/promises";
import { openAsBlob } from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import type { components } from "../src/lib/http-commons/schema.d.ts";
import { selectProfile, syncAssets } from "./assets-sync.ts";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = path.resolve(webRoot, "..");

const baseURL = process.env.LUMILIO_DEMO_BASE_URL ?? "http://localhost:6680";
const username = process.env.LUMILIO_DEMO_USERNAME ?? "lumilio-demo";
const password = process.env.LUMILIO_DEMO_PASSWORD ?? "Lumilio-Demo-2026!";
const repositoryName = process.env.LUMILIO_DEMO_REPOSITORY ?? "Lumilio Demo";

type DemoOptions = {
  concurrency: number;
  timeoutMs: number;
};

type ApiOptions = {
  method?: string;
  body?: string;
  token?: string;
  form?: FormData;
};

type Repository = {
  id: string;
  name: string;
};

type CatalogAsset = {
  id: string;
  path: string;
  sha256: string;
  bytes: number;
  expected?: { title: string; album: string; artist: string };
  source?: { albumUrl?: string };
};

type DemoAsset = CatalogAsset & {
  absolutePath: string;
};

function parseOptions(args: string[]): DemoOptions {
  const options = { concurrency: 4, timeoutMs: 20 * 60 * 1000 };
  for (let index = 0; index < args.length; index += 1) {
    const [flag, inline] = args[index].split("=");
    const value = inline ?? args[++index];
    if (flag === "--concurrency") options.concurrency = Number(value);
    else if (flag === "--timeout") options.timeoutMs = Number(value) * 1000;
    else throw new Error(`unknown argument: ${args[index]}`);
  }
  if (
    !Number.isInteger(options.concurrency) ||
    options.concurrency < 1 ||
    options.concurrency > 8
  ) {
    throw new Error("--concurrency must be between 1 and 8");
  }
  if (!Number.isFinite(options.timeoutMs) || options.timeoutMs <= 0) {
    throw new Error("--timeout must be a positive number of seconds");
  }
  return options;
}

async function api<T = Record<string, unknown>>(
  pathname: string,
  { method = "GET", body, token, form }: ApiOptions = {},
): Promise<T> {
  const request: RequestInit = {
    method,
    headers: {
      ...(form ? {} : { "content-type": "application/json" }),
      ...(token ? { authorization: `Bearer ${token}` } : {}),
    },
  };
  const requestBody = form ?? body;
  if (requestBody !== undefined) request.body = requestBody;
  const response = await fetch(`${baseURL}${pathname}`, request);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`${method} ${pathname}: ${response.status} ${JSON.stringify(payload)}`);
  }
  return payload as T;
}

/** Brings an empty instance up to an authenticated admin session. */
async function ensureAdmin(): Promise<{ token: string }> {
  const status = await api<{ admin_initialized: boolean }>("/api/v1/setup/status");

  if (!status.admin_initialized) {
    return api<{ token: string }>("/api/v1/auth/register/start", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  }
  return api<{ token: string }>("/api/v1/auth/login", {
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
async function ensureRepository(token: string): Promise<Repository> {
  const status = await api<{ primary_repository_initialized: boolean }>("/api/v1/setup/status");

  if (!status.primary_repository_initialized) {
    try {
      const { repository } = await api<{ repository: Repository }>("/api/v1/repositories", {
        method: "POST",
        token,
        body: JSON.stringify({
          name: repositoryName,
          role: "primary",
          storage_strategy: "date",
        }),
      });
      return repository;
    } catch (error) {
      // A primary created concurrently or by a prior partial run is fine; fall
      // through to discover and reuse it below.
      if (!String(error).includes("primary_exists")) throw error;
    }
  }

  const { repositories } = await api<{ repositories: Repository[] }>("/api/v1/repositories", {
    token,
  });
  const existing = repositories?.find((candidate: Repository) => candidate.name === repositoryName);
  if (existing) return existing;

  const { repository } = await api<{ repository: Repository }>("/api/v1/repositories", {
    method: "POST",
    token,
    body: JSON.stringify({
      name: repositoryName,
      role: "regular",
      storage_strategy: "date",
      directory_name: repositoryName,
    }),
  });
  return repository;
}

async function uploadAll(
  assets: DemoAsset[],
  token: string,
  repositoryId: string,
  concurrency: number,
): Promise<{ failures: string[]; receipts: string[] }> {
  const receipts: string[] = [];
  let next = 0;
  let done = 0;
  const failures: string[] = [];

  async function worker() {
    while (next < assets.length) {
      const asset = assets[next++];
      const form = new FormData();
      form.append("file", await openAsBlob(asset.absolutePath), path.basename(asset.path));
      form.append("repository_id", repositoryId);
      try {
        const response = await api<components["schemas"]["dto.UploadResponseDTO"]>(
          "/api/v1/assets",
          { method: "POST", token, form },
        );
        if (!response.receipt_id) throw new Error("upload returned no ingestion receipt");
        receipts.push(response.receipt_id);
      } catch (error) {
        failures.push(`${asset.id}: ${error instanceof Error ? error.message : String(error)}`);
      }
      done += 1;
      if (done % 25 === 0 || done === assets.length) {
        console.log(`  uploaded ${done}/${assets.length}`);
      }
    }
  }

  await Promise.all(Array.from({ length: concurrency }, worker));
  return { failures, receipts };
}

/** Wait for this run's exact receipts, including server-side duplicate resolution. */
async function waitForIngestion(
  token: string,
  receipts: string[],
  timeoutMs: number,
): Promise<void> {
  const pending = new Set(receipts);
  const deadline = Date.now() + timeoutMs;
  while (pending.size > 0 && Date.now() < deadline) {
    const ids = [...pending];
    for (let offset = 0; offset < ids.length; offset += 100) {
      const batch = ids.slice(offset, offset + 100);
      const response = await api<components["schemas"]["dto.UploadOperationStatusResponseDTO"]>(
        `/api/v1/assets/batch/operations?receipt_ids=${batch.join(",")}`,
        { token },
      );
      for (const operation of response.operations ?? []) {
        if (!operation.receipt_id || !pending.has(operation.receipt_id)) continue;
        if (!operation.terminal) continue;
        if (!operation.success) {
          throw new Error(
            `ingestion failed for ${operation.file_name}: ${JSON.stringify(operation.problem ?? operation.status)}`,
          );
        }
        pending.delete(operation.receipt_id);
      }
    }
    console.log(`  ingested ${receipts.length - pending.size}/${receipts.length}`);
    if (pending.size > 0) await new Promise<void>((resolve) => setTimeout(resolve, 2000));
  }
  if (pending.size > 0) {
    throw new Error(
      `ingestion timed out with ${pending.size} receipts pending: ${[...pending].slice(0, 5).join(", ")}`,
    );
  }
}

/** Source album URLs define demo releases; titles alone never identify a release. */
async function prepareMusic(assets: DemoAsset[], token: string, timeoutMs: number) {
  type Track = components["schemas"]["dto.MusicTrackDTO"];
  type Album = components["schemas"]["dto.MusicAlbumDTO"];
  const groups = new Map<string, { asset: DemoAsset; track: Track }[]>();
  const deadline = Date.now() + timeoutMs;
  for (const asset of assets.filter((asset) => asset.source?.albumUrl && asset.expected)) {
    let track: Track | undefined;
    while (Date.now() < deadline) {
      const response = await api<components["schemas"]["dto.MusicTrackPageDTO"]>(
        `/api/v1/music/tracks?query=${encodeURIComponent(path.basename(asset.path))}&limit=100`,
        { token },
      );
      const matches =
        response.items?.filter(
          (candidate) =>
            candidate.original_filename === path.basename(asset.path) &&
            candidate.title === asset.expected!.title &&
            candidate.album_title === asset.expected!.album,
        ) ?? [];
      if (matches.length > 1) throw new Error(`ambiguous demo Music track: ${asset.id}`);
      track = matches[0];
      if (track) break;
      await new Promise<void>((resolve) => setTimeout(resolve, 1000));
    }
    if (!track) throw new Error(`Music metadata timed out: ${asset.id}`);
    const key = asset.source!.albumUrl!;
    groups.set(key, [...(groups.get(key) ?? []), { asset, track }]);
  }
  for (const members of groups.values()) {
    // Reuse prior assignments on a retry and preserve subsequent user edits.
    const assigned = members.find(({ track }) => track.album_id)?.track.album_id;
    const first = members[0];
    let albumId = assigned;
    if (!albumId) {
      const album = await api<Album>("/api/v1/music/albums", {
        method: "POST",
        token,
        body: JSON.stringify({
          title: first.asset.expected!.album,
          artist_names: [first.asset.expected!.artist],
        }),
      });
      if (!album.album_id) throw new Error("Music album creation returned no ID");
      albumId = album.album_id;
    }
    for (const { track } of members.filter(({ track }) => !track.album_id)) {
      await api(`/api/v1/music/tracks/${track.track_id}/album`, {
        method: "PUT",
        token,
        body: JSON.stringify({ album_id: albumId, revision: track.revision }),
      });
    }
  }
  console.log(`Music ready: ${groups.size} source albums`);
}

async function main() {
  const options = parseOptions(process.argv.slice(2));
  const lockPath = path.join(repositoryRoot, "assets.lock.json");

  const { target, cached } = await syncAssets({
    lockPath,
    cacheRoot: path.join(repositoryRoot, ".cache/lumilio-assets"),
    profileName: "demo",
  });
  console.log(`${cached ? "Verified cached" : "Synchronized"} demo assets at ${target}`);

  const catalog = JSON.parse(await readFile(path.join(target, "assets.json"), "utf8")) as {
    schemaVersion: number;
    assets: CatalogAsset[];
  };
  const profile = JSON.parse(await readFile(path.join(target, "profiles/demo.json"), "utf8")) as {
    schemaVersion: number;
    name: string;
    assets: string[];
  };
  const assets: DemoAsset[] = selectProfile(catalog, profile, "demo").map((asset) => ({
    ...catalog.assets.find((candidate) => candidate.id === asset.id)!,
    absolutePath: path.join(target, asset.path),
  }));

  const { token } = await ensureAdmin();
  const repository = await ensureRepository(token);
  console.log(`Uploading ${assets.length} assets at concurrency ${options.concurrency}`);
  const { failures, receipts } = await uploadAll(assets, token, repository.id, options.concurrency);
  if (failures.length > 0) {
    throw new Error(`${failures.length} uploads failed:\n  ${failures.slice(0, 5).join("\n  ")}`);
  }

  await waitForIngestion(token, receipts, options.timeoutMs);
  await prepareMusic(assets, token, options.timeoutMs);
  console.log(`Demo library ready: ${assets.length} verified imports in ${repository.name}`);
  console.log(`Sign in as ${username} at ${baseURL}`);
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});

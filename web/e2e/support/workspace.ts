import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { api } from "./api";
import { smokeAsset, SMOKE_SCAN_ASSET, SMOKE_UPLOAD_ASSET } from "./assets";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const compose = ["compose", "-f", "docker-compose.e2e.yml", "-p", "lumilio-photos-e2e"];

/** Bootstrap admin created by `e2e/support/seed.mjs`. */
const bootstrap = {
  username: process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin",
  password: process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!",
};

export type Workspace = {
  username: string;
  password: string;
  repositoryName: string;
  scanFilename: string;
  /** Absolute path of the shared source image the upload spec sends. */
  uploadSource: string;
  /** Per-worker name it is uploaded under, so assertions cannot cross workers. */
  uploadFilename: string;
};

type Auth = { token: string };
type User = { user_id: number; username: string };
type Repository = { id: string; name: string; path: string };

async function ensureWorkerAdmin(index: number, adminToken: string) {
  const username = `e2e-w${index}`;
  const body = JSON.stringify({ username, password: bootstrap.password });

  await api<Auth>("/api/v1/auth/register/start", { method: "POST", body }).catch((error: Error) => {
    // A retried worker reuses the account it registered on the first attempt.
    if (!error.message.includes("409")) throw error;
  });

  const { users } = await api<{ users: User[] }>("/api/v1/users", { token: adminToken });
  const user = users.find((candidate) => candidate.username === username);
  if (!user) throw new Error(`worker user ${username} was not created`);

  // Self-service registration only grants admin to the first account, and
  // repository and scan endpoints require admin, so promote the worker's user.
  await api(`/api/v1/users/${user.user_id}`, {
    method: "PATCH",
    token: adminToken,
    body: JSON.stringify({ role: "admin" }),
  });

  return username;
}

async function ensureWorkerRepository(index: number, token: string) {
  const name = `E2E Worker ${index}`;
  const { repositories } = await api<{ repositories: Repository[] }>("/api/v1/repositories", {
    token,
  });
  const existing = repositories?.find((repository) => repository.name === name);
  if (existing) return existing;

  // Regular, not primary: `repositories_one_primary_idx` allows a single primary
  // repository for the whole instance.
  const { repository } = await api<{ repository: Repository }>("/api/v1/repositories", {
    method: "POST",
    token,
    body: JSON.stringify({
      name,
      role: "regular",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  return repository;
}

function placeScanFixture(repository: Repository, source: string, scanFilename: string) {
  // Storage is a named volume, so the fixture goes in through the container.
  const result = spawnSync(
    "docker",
    [...compose, "cp", source, `server:${repository.path}/${scanFilename}`],
    { cwd: repositoryRoot, stdio: "inherit" },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`docker compose cp failed (${result.status})`);
}

/**
 * Gives one Playwright worker its own admin user and repository, so parallel
 * workers never scan or upload into the same mutable state.
 */
export async function provisionWorkspace(index: number): Promise<Workspace> {
  const { token: adminToken } = await api<Auth>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(bootstrap),
  });

  const username = await ensureWorkerAdmin(index, adminToken);
  const { token } = await api<Auth>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password: bootstrap.password }),
  });

  const repository = await ensureWorkerRepository(index, token);
  // The smoke profile is deliberately minimal — one scan and one upload asset —
  // so workers share the source bytes and separate themselves by filename,
  // repository and owner instead.
  const scanFilename = `e2e-scan-w${index}.jpg`;
  placeScanFixture(repository, smokeAsset(SMOKE_SCAN_ASSET), scanFilename);

  return {
    username,
    password: bootstrap.password,
    repositoryName: repository.name,
    scanFilename,
    uploadSource: smokeAsset(SMOKE_UPLOAD_ASSET),
    uploadFilename: `e2e-upload-w${index}.jpg`,
  };
}

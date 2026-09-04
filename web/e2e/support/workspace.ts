import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { api } from "./api";
import { compose, docker, repositoryRoot } from "./docker.ts";
import { isMFAInvalidError, loadBootstrapTOTP, nextTOTPCode, totpCode } from "./totp";
import {
  AUTH_ISOLATION_ASSET,
  smokeAsset,
  SMOKE_SCAN_ASSET,
  SMOKE_UPLOAD_ASSET,
  SMOKE_VIDEO_ASSET,
} from "./assets";

/** Bootstrap admin created by `e2e/support/seed.ts`. */
const bootstrap = {
  username: process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin",
  password: process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!",
};

export type Workspace = {
  username: string;
  password: string;
  token: string;
  repositoryId: string;
  repositoryName: string;
  scanFilename: string;
  /** Absolute path of the shared source image the upload spec sends. */
  uploadSource: string;
  /** Per-attempt name it is uploaded under, so assertions cannot cross attempts. */
  uploadFilename: string;
  /** Distinct real image reserved for auth/session isolation assertions. */
  authIsolationSource: string;
  videoSource: string;
  videoFilename: string;
};

type Auth = {
  token?: string;
  requires_mfa?: boolean;
  mfa_token?: string;
};
type User = { user_id: number; username: string };
type Repository = { id: string; name: string; path: string };

async function ensureAttemptAdmin(index: number, adminToken: string) {
  const username = `e2e-a${index}`;
  const body = JSON.stringify({ username, password: bootstrap.password });

  await api<Auth>("/api/v1/auth/register/start", { method: "POST", body }).catch((error: Error) => {
    // A repeated invocation for this exact attempt may reuse its account.
    if (!error.message.includes("409")) throw error;
  });

  const { users } = await api<{ users: User[] }>("/api/v1/users", { token: adminToken });
  const user = users.find((candidate) => candidate.username === username);
  if (!user) throw new Error(`attempt user ${username} was not created`);

  // Self-service registration only grants admin to the first account, and
  // repository and scan endpoints require admin, so promote the attempt's user.
  await api(`/api/v1/users/${user.user_id}`, {
    method: "PATCH",
    token: adminToken,
    body: JSON.stringify({ role: "admin" }),
  });

  return username;
}

async function loginBootstrap(): Promise<{ token: string }> {
  const login = await api<Auth>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(bootstrap),
  });
  if (login.token) return { token: login.token };
  if (!login.requires_mfa || !login.mfa_token) {
    throw new Error("bootstrap admin login did not return a session or an MFA challenge");
  }
  const bootstrapTOTP = loadBootstrapTOTP();
  if (!bootstrapTOTP || bootstrapTOTP.username !== bootstrap.username) {
    throw new Error("bootstrap admin requires MFA but the temporary E2E TOTP hand-off is missing");
  }
  const verify = (code: string) =>
    api<Auth>("/api/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ mfa_token: login.mfa_token, code, method: "totp" }),
    });
  const code = totpCode(bootstrapTOTP.secret);
  let verified: Auth;
  try {
    verified = await verify(code);
  } catch (error) {
    // The seed process may have consumed this thirty-second counter moments
    // earlier. Replay rejection is expected; any other failure still aborts.
    if (!isMFAInvalidError(error)) throw error;
    verified = await verify(await nextTOTPCode(bootstrapTOTP.secret, code));
  }
  if (!verified.token) throw new Error("bootstrap admin MFA verification did not return a session");
  return { token: verified.token };
}

async function ensureAttemptRepository(index: number, token: string) {
  const name = `E2E Attempt ${index}`;
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
      directory_name: name,
      role: "regular",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  return repository;
}

function placeScanFixture(repository: Repository, source: string, scanFilename: string) {
  // ROE assigns scanned files to the bootstrap owner, so distinct users and
  // paths alone do not isolate exact-content deduplication. Add a JPEG comment
  // without changing pixels or the pinned source, giving each attempt its own
  // content identity and therefore its own original filename.
  const original = readFileSync(source);
  if (original[0] !== 0xff || original[1] !== 0xd8) {
    throw new Error("scan fixture must be a JPEG");
  }
  const comment = Buffer.from(scanFilename, "utf8");
  const marker = Buffer.alloc(4);
  marker.writeUInt16BE(0xfffe, 0);
  marker.writeUInt16BE(comment.length + 2, 2);
  const temporary = mkdtempSync(join(tmpdir(), "lumilio-scan-attempt-"));
  const attemptSource = join(temporary, scanFilename);
  try {
    writeFileSync(
      attemptSource,
      Buffer.concat([original.subarray(0, 2), marker, comment, original.subarray(2)]),
    );
    // Storage is a named volume, so the fixture goes in through the container.
    const result = spawnSync(
      "docker",
      [...docker, ...compose, "cp", attemptSource, `lumilio:${repository.path}/${scanFilename}`],
      { cwd: repositoryRoot, stdio: "inherit" },
    );
    if (result.error) throw result.error;
    if (result.status !== 0) throw new Error(`docker compose cp failed (${result.status})`);
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}

/**
 * Gives one Playwright attempt its own admin user, repository and filenames.
 * The caller derives `index` from test/repeat/retry identity, so a retry never
 * inherits mutable catalog state from an earlier failed attempt.
 */
export async function provisionWorkspace(index: number): Promise<Workspace> {
  const { token: adminToken } = await loginBootstrap();

  const username = await ensureAttemptAdmin(index, adminToken);
  const workerAuth = await api<Auth>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password: bootstrap.password }),
  });
  if (!workerAuth.token) throw new Error(`attempt user ${username} login did not return a session`);
  const { token } = workerAuth;

  const repository = await ensureAttemptRepository(index, token);
  // The smoke profile is deliberately minimal — one scan and one upload asset —
  // so derive a pixel-identical, content-distinct scan copy for each attempt.
  const scanFilename = `e2e-scan-a${index}.jpg`;
  placeScanFixture(repository, smokeAsset(SMOKE_SCAN_ASSET), scanFilename);

  return {
    username,
    password: bootstrap.password,
    token,
    repositoryId: repository.id,
    repositoryName: repository.name,
    scanFilename,
    uploadSource: smokeAsset(SMOKE_UPLOAD_ASSET),
    uploadFilename: `e2e-upload-a${index}.jpg`,
    authIsolationSource: smokeAsset(AUTH_ISOLATION_ASSET),
    videoSource: smokeAsset(SMOKE_VIDEO_ASSET),
    videoFilename: `e2e-video-a${index}.mp4`,
  };
}

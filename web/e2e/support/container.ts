import { execFileSync, spawnSync } from "node:child_process";
import { baseURL } from "./api.ts";
import { compose, docker, repositoryRoot } from "./docker.ts";

/**
 * Runs one command inside the running `lumilio` service and returns its
 * trimmed stdout. Storage and app state are named volumes, so every file fact
 * a spec needs is read through the container, never from the host. A Docker or
 * container failure throws instead of reading as an absent file.
 */
export function container(...command: string[]): string {
  return execFileSync("docker", [...docker, ...compose, "exec", "-T", "lumilio", ...command], {
    cwd: repositoryRoot,
    encoding: "utf8",
  }).trim();
}

/** SHA-256 of one file inside the container. */
export function containerChecksum(path: string): string {
  return container("sha256sum", path).split(" ")[0] ?? "";
}

/** The regular files below a directory inside the container. */
export function containerFiles(directory: string): string[] {
  return container("find", directory, "-type", "f").split("\n").filter(Boolean);
}

function composeControl(...args: string[]): void {
  const result = spawnSync("docker", [...docker, ...compose, ...args], {
    cwd: repositoryRoot,
    stdio: "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`docker compose ${args.join(" ")} failed (${result.status})`);
  }
}

/**
 * Waits until the Server answers its readiness probe again, after a restart or
 * a reinstall. Transport errors are the expected intermediate state.
 */
export async function waitForServerReady(timeoutMs = 180_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastFailure = "no attempt yet";
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${baseURL}/api/v1/health/ready`);
      if (response.ok) return;
      lastFailure = `status ${response.status}`;
    } catch (error) {
      lastFailure = error instanceof Error ? error.message : String(error);
    }
    await new Promise((resolve) => setTimeout(resolve, 1_000));
  }
  throw new Error(`the Server did not become ready within ${timeoutMs}ms (${lastFailure})`);
}

/** Restarts the Server process; volumes, catalog and secrets stay. */
export async function restartServer(): Promise<void> {
  composeControl("restart", "lumilio");
  await waitForServerReady();
}

/**
 * Models a reinstall over kept media: the Server stops, its app-state volume
 * (catalog, queue, secrets, logs, backups) is emptied through a one-off
 * container of the same image, and the Server starts again against the
 * untouched storage volume. Nothing under /data/storage is read or written.
 */
export async function reinstallKeepingStorage(): Promise<void> {
  composeControl("stop", "lumilio");
  composeControl(
    "run",
    "--rm",
    "-T",
    "--no-deps",
    "--user",
    "app",
    "--entrypoint",
    "sh",
    "lumilio",
    "-c",
    "find /data/app-state -mindepth 1 -delete",
  );
  composeControl("start", "lumilio");
  await waitForServerReady();
}

import { randomBytes } from "node:crypto";
import { chmod, copyFile, mkdir, rm, writeFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const cache = path.join(root, ".cache/e2e");
const compose = ["compose", "-f", "docker-compose.e2e.yml", "-p", "lumilio-photos-e2e"];

function run(args) {
  const result = spawnSync("docker", args, { cwd: root, stdio: "inherit", env: process.env });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`docker ${args.join(" ")} failed (${result.status})`);
}

const command = process.argv[2];
if (command === "up") {
  // The bootstrap password below is regenerated per run, but PostgreSQL only
  // applies it while initializing an empty data directory. Reusing a volume
  // would therefore fail authentication, so every `up` starts from a clean one
  // — which is also the empty database each E2E job is supposed to get.
  run([...compose, "down", "--volumes", "--remove-orphans"]);
  await rm(cache, { recursive: true, force: true });
  // Storage is bind-mounted, so the image's `chown app:app /data` is shadowed by
  // whatever the host directory looks like. The server runs as uid 10001 and the
  // seed writes the scan fixture from the host, so both need to write here and
  // their uids do not match on Linux. Create the directories up front and open
  // them, rather than letting whoever gets there first own them. mkdir's mode is
  // masked by umask, hence the explicit chmod.
  for (const dir of ["storage", "storage/primary"]) {
    const target = path.join(cache, dir);
    await mkdir(target, { recursive: true });
    await chmod(target, 0o777);
  }
  // World-readable on purpose: the server container runs as a non-root user and
  // plain Compose bind-mounts secret files as-is, with no uid/gid/mode option to
  // fix ownership. On Linux CI, host and container UIDs map directly, so 0600
  // leaves the container unable to read its own bootstrap password. The value is
  // random per run and lives in an ignored cache directory.
  await writeFile(path.join(cache, "db_bootstrap_password"), randomBytes(32).toString("hex"), {
    mode: 0o644,
  });
  const npmrc = process.env.LUMILIO_E2E_NPMRC ?? path.join(process.env.HOME ?? "", ".npmrc");
  await copyFile(npmrc, path.join(cache, "npmrc"));
  run([...compose, "up", "-d", "--build", "--wait"]);
} else if (command === "down") {
  run([...compose, "down", "--volumes", "--remove-orphans"]);
  await rm(cache, { recursive: true, force: true });
} else if (command === "logs") {
  run([...compose, "logs", "--no-color"]);
} else {
  throw new Error("usage: environment.mjs <up|down|logs>");
}

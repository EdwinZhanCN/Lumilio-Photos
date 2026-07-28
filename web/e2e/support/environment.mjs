import { copyFile, mkdir, rm } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const cache = path.join(root, ".cache/e2e");
// CI layers a build-cache override on top through this env var; local runs leave
// it unset and build without a cache backend.
const extraFile = process.env.LUMILIO_E2E_COMPOSE_EXTRA;
const composeFile = path.join(root, "web/e2e/compose.yml");
const extraPath = extraFile
  ? path.isAbsolute(extraFile)
    ? extraFile
    : path.resolve(root, extraFile)
  : undefined;
const compose = [
  "compose",
  "-f",
  composeFile,
  ...(extraPath ? ["-f", extraPath] : []),
  "-p",
  "lumilio-photos-e2e",
];

function run(args) {
  const result = spawnSync("docker", args, { cwd: root, stdio: "inherit", env: process.env });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`docker ${args.join(" ")} failed (${result.status})`);
}

const command = process.argv[2];
if (command === "up") {
  // Every run starts from clean media and app-state volumes, including a fresh
  // SQLite catalog.
  run([...compose, "down", "--volumes", "--remove-orphans"]);
  await rm(cache, { recursive: true, force: true });
  await mkdir(cache, { recursive: true });
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

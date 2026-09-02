import { mkdir, rm } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { compose, docker, repositoryRoot } from "./docker.ts";

const cache = path.join(repositoryRoot, ".cache/e2e");

function run(args: string[]): void {
  const result = spawnSync("docker", [...docker, ...args], {
    cwd: repositoryRoot,
    stdio: "inherit",
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`docker ${[...docker, ...args].join(" ")} failed (${result.status})`);
  }
}

const command = process.argv[2];
if (command === "up") {
  // Every run starts from clean media and app-state volumes, including a fresh
  // SQLite catalog.
  run([...compose, "down", "--volumes", "--remove-orphans"]);
  await rm(cache, { recursive: true, force: true });
  await mkdir(cache, { recursive: true });
  run([...compose, "up", "-d", "--build", "--wait"]);
} else if (command === "down") {
  run([...compose, "down", "--volumes", "--remove-orphans"]);
  await rm(cache, { recursive: true, force: true });
} else if (command === "logs") {
  run([...compose, "logs", "--no-color"]);
} else {
  throw new Error("usage: environment.ts <up|down|logs>");
}

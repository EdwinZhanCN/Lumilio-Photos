import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = path.resolve(webRoot, "..");

function sha256(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    encoding: "utf8",
    env: { ...process.env, ...options.env },
    stdio: options.capture ? ["ignore", "pipe", "pipe"] : "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    const details = options.capture ? `\n${result.stderr.trim()}` : "";
    throw new Error(
      `${command} ${args.join(" ")} failed with exit code ${result.status}${details}`,
    );
  }
  return options.capture ? result.stdout : "";
}

export function parseLock(raw) {
  const lock = JSON.parse(raw);
  if (lock.schemaVersion !== 1) throw new Error("assets.lock.json must use schemaVersion 1");
  if (!/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\.git$/.test(lock.repository)) {
    throw new Error("assets.lock.json repository must be an HTTPS GitHub .git URL");
  }
  if (!/^[a-f0-9]{40}$/.test(lock.revision)) {
    throw new Error("assets.lock.json revision must be a full 40-character commit SHA");
  }
  if (!/^[a-f0-9]{64}$/.test(lock.manifestSha256)) {
    throw new Error("assets.lock.json manifestSha256 must be a lowercase SHA-256");
  }
  if (!/^[a-z0-9][a-z0-9-]*$/.test(lock.profile)) {
    throw new Error("assets.lock.json profile is invalid");
  }
  return lock;
}

export function selectProfile(catalog, profile, profileName) {
  if (catalog.schemaVersion !== 1 || !Array.isArray(catalog.assets)) {
    throw new Error("assets.json must contain a schemaVersion 1 assets array");
  }
  if (
    profile.schemaVersion !== 1 ||
    profile.name !== profileName ||
    !Array.isArray(profile.assets)
  ) {
    throw new Error(`profiles/${profileName}.json is invalid`);
  }

  const catalogById = new Map();
  for (const asset of catalog.assets) {
    if (!asset.id || catalogById.has(asset.id))
      throw new Error(`duplicate or missing asset ID: ${asset.id}`);
    if (
      !asset.path?.startsWith("media/") ||
      path.posix.normalize(asset.path) !== asset.path ||
      ["\0", "\r", "\n", ",", "*", "?", "[", "]", "\\"].some((character) =>
        asset.path.includes(character),
      )
    ) {
      throw new Error(`invalid media path for ${asset.id}`);
    }
    if (!/^[a-f0-9]{64}$/.test(asset.sha256) || !Number.isSafeInteger(asset.bytes)) {
      throw new Error(`invalid integrity metadata for ${asset.id}`);
    }
    catalogById.set(asset.id, asset);
  }

  if (new Set(profile.assets).size !== profile.assets.length) {
    throw new Error(`profiles/${profileName}.json contains duplicate IDs`);
  }
  return profile.assets.map((id) => {
    const asset = catalogById.get(id);
    if (!asset) throw new Error(`profiles/${profileName}.json references unknown asset ${id}`);
    return asset;
  });
}

export async function validateMaterializedAssets(root, assets) {
  for (const asset of assets) {
    const file = path.join(root, asset.path);
    const bytes = await readFile(file);
    if ((await stat(file)).size !== asset.bytes)
      throw new Error(`byte-size mismatch for ${asset.id}`);
    if (sha256(bytes) !== asset.sha256) throw new Error(`SHA-256 mismatch for ${asset.id}`);
  }
}

function readGitFile(gitRoot, revision, relativePath) {
  return run("git", ["show", `${revision}:${relativePath}`], { cwd: gitRoot, capture: true });
}

function parseProfileArgument(args, fallback) {
  let profile = fallback;
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--") continue;
    if (argument.startsWith("--profile=")) profile = argument.slice("--profile=".length);
    else if (argument === "--profile") profile = args[++index];
    else throw new Error(`unknown argument: ${argument}`);
  }
  if (!/^[a-z0-9][a-z0-9-]*$/.test(profile ?? "")) throw new Error("profile is invalid");
  return profile;
}

async function validateCache(target, lock, profileName) {
  const metadataPath = path.join(target, ".lumilio-assets-sync.json");
  if (!existsSync(metadataPath)) return false;
  const metadata = JSON.parse(await readFile(metadataPath, "utf8"));
  if (
    metadata.revision !== lock.revision ||
    metadata.profile !== profileName ||
    metadata.manifestSha256 !== lock.manifestSha256
  ) {
    return false;
  }
  const manifestBytes = await readFile(path.join(target, "assets.json"));
  if (sha256(manifestBytes) !== lock.manifestSha256) return false;
  const catalog = JSON.parse(manifestBytes);
  const profile = JSON.parse(
    await readFile(path.join(target, `profiles/${profileName}.json`), "utf8"),
  );
  const assets = selectProfile(catalog, profile, profileName);
  await validateMaterializedAssets(target, assets);
  return true;
}

export async function syncAssets({ lockPath, cacheRoot, profileName }) {
  const lock = parseLock(await readFile(lockPath, "utf8"));
  const selectedProfile = profileName ?? lock.profile;
  if (!/^[a-z0-9][a-z0-9-]*$/.test(selectedProfile)) throw new Error("profile is invalid");

  const target = path.join(cacheRoot, lock.revision, selectedProfile);
  try {
    if (await validateCache(target, lock, selectedProfile)) {
      return { target, cached: true };
    }
  } catch {
    // Invalid or incomplete caches are replaced atomically below.
  }

  await mkdir(path.dirname(target), { recursive: true });
  const temporary = `${target}.tmp-${process.pid}-${Date.now()}`;
  await rm(temporary, { recursive: true, force: true });
  await mkdir(temporary, { recursive: true });

  try {
    run("git", ["init", "--quiet"], { cwd: temporary });
    run("git", ["remote", "add", "origin", lock.repository], { cwd: temporary });
    run("git", ["fetch", "--quiet", "--depth=1", "origin", lock.revision], {
      cwd: temporary,
      env: { GIT_LFS_SKIP_SMUDGE: "1" },
    });

    const manifestRaw = readGitFile(temporary, "FETCH_HEAD", "assets.json");
    if (sha256(manifestRaw) !== lock.manifestSha256) {
      throw new Error("assets.json does not match assets.lock.json manifestSha256");
    }
    const profileRaw = readGitFile(temporary, "FETCH_HEAD", `profiles/${selectedProfile}.json`);
    const catalog = JSON.parse(manifestRaw);
    const profile = JSON.parse(profileRaw);
    const assets = selectProfile(catalog, profile, selectedProfile);

    run("git", ["sparse-checkout", "init", "--no-cone"], { cwd: temporary });
    const sparsePatterns = [
      "/assets.json",
      `/profiles/${selectedProfile}.json`,
      ...assets.map((asset) => `/${asset.path}`),
    ];
    await writeFile(
      path.join(temporary, ".git/info/sparse-checkout"),
      `${sparsePatterns.join("\n")}\n`,
    );
    run("git", ["checkout", "--quiet", "--detach", "FETCH_HEAD"], {
      cwd: temporary,
      env: { GIT_LFS_SKIP_SMUDGE: "1" },
    });
    run(
      "git",
      ["lfs", "pull", "--include", assets.map((asset) => asset.path).join(","), "--exclude", ""],
      {
        cwd: temporary,
      },
    );
    await validateMaterializedAssets(temporary, assets);
    await writeFile(
      path.join(temporary, ".lumilio-assets-sync.json"),
      `${JSON.stringify(
        {
          schemaVersion: 1,
          repository: lock.repository,
          revision: lock.revision,
          profile: selectedProfile,
          manifestSha256: lock.manifestSha256,
          assets: assets.map((asset) => asset.id),
        },
        null,
        2,
      )}\n`,
    );

    await rm(target, { recursive: true, force: true });
    await rename(temporary, target);
    return { target, cached: false };
  } catch (error) {
    await rm(temporary, { recursive: true, force: true });
    throw error;
  }
}

async function main() {
  const lockPath = path.join(repositoryRoot, "assets.lock.json");
  const lock = parseLock(await readFile(lockPath, "utf8"));
  const profileName = parseProfileArgument(process.argv.slice(2), lock.profile);
  const result = await syncAssets({
    lockPath,
    cacheRoot: path.join(repositoryRoot, ".cache/lumilio-assets"),
    profileName,
  });
  console.log(
    `${result.cached ? "Verified cached" : "Synchronized"} ${profileName} assets at ${result.target}`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });
}

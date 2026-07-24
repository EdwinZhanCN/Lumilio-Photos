import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

const lock: { revision: string; profile: string } = JSON.parse(
  readFileSync(path.join(repositoryRoot, "assets.lock.json"), "utf8"),
);

// `assets.json` catalogues the whole asset repository, but `assets:sync` only
// materialises the ids the profile references. Resolving through the profile
// keeps this to files that are actually on disk.
export function profileAsset(profileName: string, id: string): string {
  const profileRoot = path.join(
    repositoryRoot,
    ".cache/lumilio-assets",
    lock.revision,
    profileName,
  );
  const profile: { assets: string[] } = JSON.parse(
    readFileSync(path.join(profileRoot, "profiles", `${profileName}.json`), "utf8"),
  );
  const catalog: { assets: { id: string; path: string }[] } = JSON.parse(
    readFileSync(path.join(profileRoot, "assets.json"), "utf8"),
  );

  if (!profile.assets.includes(id)) {
    throw new Error(`asset ${id} is not in the ${profileName} profile`);
  }
  const asset = catalog.assets.find((candidate) => candidate.id === id);
  if (!asset) throw new Error(`asset ${id} is missing from the catalogue`);
  return path.join(profileRoot, asset.path);
}

/** Absolute path of one smoke-profile asset, addressed by its stable id. */
export function smokeAsset(id: string): string {
  return profileAsset(lock.profile, id);
}

export const SMOKE_SCAN_ASSET = "picsum-scan-000";
export const SMOKE_UPLOAD_ASSET = "picsum-upload-123";
export const SMOKE_VIDEO_ASSET = "commons-video-ocean-waves";
export const VIDEO_REGRESSION_PROFILE = "e2e";
export const VIDEO_REGRESSION_PHOTO_ASSET = "picsum-upload-124";
export const VIDEO_REGRESSION_ASSETS = [
  "commons-video-ocean-waves",
  "commons-video-chameleon-flowers",
  "commons-video-mountain-landscape",
] as const;

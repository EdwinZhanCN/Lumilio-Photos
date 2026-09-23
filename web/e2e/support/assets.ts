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
export const AUTH_ISOLATION_ASSET = "landing-01-lake-cabin";
export const VIDEO_REGRESSION_PROFILE = "e2e";
export const VIDEO_REGRESSION_PHOTO_ASSET = "picsum-upload-124";
export const VIDEO_REGRESSION_DISABLED_ASSET = "immich-video-eiffel-tower";
export const VIDEO_REGRESSION_ASSETS = [
  "commons-video-ocean-waves",
  "commons-video-chameleon-flowers",
  "commons-video-mountain-landscape",
] as const;

/**
 * Returns the JPEG at `sourcePath` with a comment segment carrying
 * `markerText`. Pixels are unchanged but content identity is unique, so an
 * upload cannot be deduplicated against another attempt's copy of the same
 * pinned source.
 */
export function uniqueJpeg(sourcePath: string, markerText: string): Buffer {
  const source = readFileSync(sourcePath);
  const endOfImage = source.lastIndexOf(Buffer.from([0xff, 0xd9]));
  if (endOfImage < 0) throw new Error(`${sourcePath} is not a JPEG`);
  const marker = Buffer.from(markerText, "utf8");
  const markerLength = marker.length + 2;
  return Buffer.concat([
    source.subarray(0, endOfImage),
    Buffer.from([0xff, 0xfe, markerLength >> 8, markerLength & 0xff]),
    marker,
    source.subarray(endOfImage),
  ]);
}

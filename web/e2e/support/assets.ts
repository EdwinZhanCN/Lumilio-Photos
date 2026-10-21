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
  return resolveAsset(profileName, profileName, id);
}

/**
 * Absolute path of one asset synced by an explicit selection
 * (`assets:sync -- --profile <name> --asset <id> …`). The selection lives in
 * its own `<profile>+selection` cache directory, and the id must be one the
 * sync actually materialised, so a partial copy never stands in for the full
 * profile that other slices resolve through `profileAsset`.
 */
export function selectedProfileAsset(profileName: string, id: string): string {
  const directory = `${profileName}+selection`;
  const sync: { assets?: string[] } = JSON.parse(
    readFileSync(
      path.join(
        repositoryRoot,
        ".cache/lumilio-assets",
        lock.revision,
        directory,
        ".lumilio-assets-sync.json",
      ),
      "utf8",
    ),
  );
  if (!sync.assets?.includes(id)) {
    throw new Error(`asset ${id} was not synced by the ${profileName} selection`);
  }
  return resolveAsset(directory, profileName, id);
}

function resolveAsset(directory: string, profileName: string, id: string): string {
  const profileRoot = path.join(repositoryRoot, ".cache/lumilio-assets", lock.revision, directory);
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

/**
 * How long a spec waits for audio to start. Starting a track waits for its
 * first bytes, and right after a seed a low-power host is still draining
 * ingestion and can take several seconds to answer.
 */
export const PLAYBACK_START_TIMEOUT = 30_000;

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
 * Portraits of two different people for the People merge regression. Only the
 * `demo` profile carries more than one of them, so the `@people` slice syncs
 * just these ids from it (`--asset`). Face recognition answers for these exact pixels are recorded
 * under `server/tools/fakelumen/fixtures/records/face_recognition/`.
 */
export const PEOPLE_PROFILE = "demo";
export const PEOPLE_PERSON_A_ASSETS = [
  "landing-07-portrait-a",
  "landing-19-portrait-a-close",
  "landing-20-portrait-a-warm",
] as const;
export const PEOPLE_PERSON_B_ASSETS = [
  "landing-08-portrait-b",
  "landing-21-portrait-b-close",
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

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  cacheDirectoryName,
  parseArguments,
  parseLock,
  selectAssets,
  selectProfile,
  validateMaterializedAssets,
} from "./assets-sync.ts";

const digest = (value: string) => createHash("sha256").update(value).digest("hex");

await test("parseLock requires immutable revision and manifest hashes", () => {
  const lock = parseLock(
    JSON.stringify({
      schemaVersion: 1,
      repository: "https://github.com/EdwinZhanCN/Lumilio-Assets.git",
      revision: "a".repeat(40),
      profile: "smoke",
      manifestSha256: "b".repeat(64),
    }),
  );
  assert.equal(lock.profile, "smoke");
  assert.throws(() => parseLock(JSON.stringify({ ...lock, revision: "main" })), /40-character/);
});

await test("selectProfile rejects unknown and duplicate asset IDs", () => {
  const asset = {
    id: "image-a",
    path: "media/image-a.jpg",
    sha256: "a".repeat(64),
    bytes: 3,
  };
  const catalog = { schemaVersion: 1, assets: [asset] };
  assert.deepEqual(
    selectProfile(catalog, { schemaVersion: 1, name: "smoke", assets: ["image-a"] }, "smoke"),
    [asset],
  );
  assert.throws(
    () => selectProfile(catalog, { schemaVersion: 1, name: "smoke", assets: ["missing"] }, "smoke"),
    /unknown asset/,
  );
  assert.throws(
    () =>
      selectProfile(
        catalog,
        { schemaVersion: 1, name: "smoke", assets: ["image-a", "image-a"] },
        "smoke",
      ),
    /duplicate IDs/,
  );
  assert.throws(
    () =>
      selectProfile(
        { schemaVersion: 1, assets: [{ ...asset, path: "media/*.jpg" }] },
        { schemaVersion: 1, name: "smoke", assets: ["image-a"] },
        "smoke",
      ),
    /invalid media path/,
  );
});

await test("validateMaterializedAssets detects altered bytes", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "lumilio-assets-test-"));
  try {
    await mkdir(path.join(root, "media"));
    await writeFile(path.join(root, "media/image.jpg"), "abc");
    const asset = {
      id: "image",
      path: "media/image.jpg",
      bytes: 3,
      sha256: digest("abc"),
    };
    await validateMaterializedAssets(root, [asset]);
    await writeFile(path.join(root, "media/image.jpg"), "abd");
    await assert.rejects(validateMaterializedAssets(root, [asset]), /SHA-256 mismatch/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

await test("parseArguments collects an explicit asset selection", () => {
  assert.deepEqual(parseArguments(["--", "--profile", "e2e"], "smoke"), { profile: "e2e" });
  assert.deepEqual(
    parseArguments(["--profile=demo", "--asset", "image-a", "--asset=image-b"], "smoke"),
    { profile: "demo", assetIds: ["image-a", "image-b"] },
  );
  assert.throws(() => parseArguments(["--asset"], "smoke"), /--asset requires a value/);
  assert.throws(() => parseArguments(["--asset", "../escape"], "smoke"), /asset ID is invalid/);
});

await test("selectAssets narrows a profile to requested members only", () => {
  const asset = (id: string) => ({
    id,
    path: `media/${id}.jpg`,
    sha256: digest(id),
    bytes: id.length,
  });
  const profile = [asset("image-a"), asset("image-b"), asset("image-c")];
  assert.deepEqual(selectAssets(profile, ["image-c", "image-a"], "demo"), [
    asset("image-c"),
    asset("image-a"),
  ]);
  assert.throws(() => selectAssets(profile, ["image-z"], "demo"), /not in the demo profile/);
  assert.throws(() => selectAssets(profile, ["image-a", "image-a"], "demo"), /duplicate/);
  // A selection never occupies the full profile's cache directory.
  assert.equal(cacheDirectoryName("demo"), "demo");
  assert.equal(cacheDirectoryName("demo", ["image-a"]), "demo+selection");
});

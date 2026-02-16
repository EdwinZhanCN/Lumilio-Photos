#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { createSign } from "node:crypto";
import { readdirSync, readFileSync, statSync, writeFileSync, mkdirSync } from "node:fs";
import { join, relative, resolve, extname } from "node:path";
import { tmpdir } from "node:os";

function parseArgs(argv) {
  const out = {};

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) continue;

    const key = token.slice(2);
    const value = argv[i + 1];
    if (!value || value.startsWith("--")) {
      out[key] = "true";
      continue;
    }

    out[key] = value;
    i += 1;
  }

  return out;
}

function usageAndExit(message) {
  if (message) {
    console.error(`Error: ${message}`);
  }

  console.error(`
Usage:
  node infra/cloudflare/scripts/publish-plugin.mjs \\
    --plugin-id com.lumilio.border \\
    --version 0.1.0 \\
    --source plugins/lumilio-border-plugin/dist \\
    --manifest plugins/lumilio-border-plugin/manifest.json \\
    --bucket lumilio-plugin-artifacts \\
    --db lumilio_plugin_registry

Optional:
  --channel stable
`);
  process.exit(1);
}

function listFilesRecursive(dir) {
  const entries = readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...listFilesRecursive(fullPath));
    } else if (entry.isFile()) {
      files.push(fullPath);
    }
  }

  return files;
}

function toBase64Url(buffer) {
  return buffer
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

function sortKeysDeep(input) {
  if (Array.isArray(input)) {
    return input.map(sortKeysDeep);
  }

  if (typeof input !== "object" || input === null) {
    return input;
  }

  const out = {};
  const keys = Object.keys(input).sort();
  for (const key of keys) {
    out[key] = sortKeysDeep(input[key]);
  }

  return out;
}

function canonicalizeManifestPayload(manifest) {
  const { signature, ...rest } = manifest;
  void signature;
  return JSON.stringify(sortKeysDeep(rest));
}

function detectMimeType(filePath) {
  const ext = extname(filePath).toLowerCase();
  switch (ext) {
    case ".mjs":
    case ".js":
      return "application/javascript";
    case ".wasm":
      return "application/wasm";
    case ".json":
      return "application/json";
    default:
      return "application/octet-stream";
  }
}

function runWrangler(args) {
  execFileSync("wrangler", args, {
    stdio: "inherit",
  });
}

function escapeSql(value) {
  return String(value).replace(/'/g, "''");
}

function maybeSignManifest(manifest) {
  const privateKeyPem = process.env.PLUGIN_SIGNING_PRIVATE_KEY_PEM;
  const keyId = process.env.PLUGIN_SIGNING_KEY_ID || manifest.signature?.keyId || "lumilio-dev-1";

  if (!privateKeyPem) {
    return manifest;
  }

  const payload = canonicalizeManifestPayload(manifest);
  const signer = createSign("SHA256");
  signer.update(payload);
  signer.end();

  const signature = signer.sign(privateKeyPem);

  return {
    ...manifest,
    signature: {
      keyId,
      algorithm: "ECDSA_P256_SHA256",
      value: toBase64Url(signature),
    },
  };
}

function main() {
  const args = parseArgs(process.argv.slice(2));

  const pluginId = args["plugin-id"];
  const version = args.version;
  const sourceDir = args.source;
  const manifestPath = args.manifest;
  const bucket = args.bucket;
  const database = args.db;
  const channel = args.channel || "stable";

  if (!pluginId) usageAndExit("--plugin-id is required");
  if (!version) usageAndExit("--version is required");
  if (!sourceDir) usageAndExit("--source is required");
  if (!manifestPath) usageAndExit("--manifest is required");
  if (!bucket) usageAndExit("--bucket is required");
  if (!database) usageAndExit("--db is required");

  const resolvedSourceDir = resolve(sourceDir);
  const resolvedManifestPath = resolve(manifestPath);

  if (!statSync(resolvedSourceDir).isDirectory()) {
    usageAndExit(`source directory not found: ${resolvedSourceDir}`);
  }

  const manifest = JSON.parse(readFileSync(resolvedManifestPath, "utf8"));
  manifest.id = pluginId;
  manifest.version = version;

  const signedManifest = maybeSignManifest(manifest);
  const tmpFolder = resolve(tmpdir(), "lumilio-plugin-publish");
  mkdirSync(tmpFolder, { recursive: true });
  const signedManifestPath = join(tmpFolder, `${pluginId}-${version}-manifest.json`);
  writeFileSync(signedManifestPath, JSON.stringify(signedManifest, null, 2));

  const files = listFilesRecursive(resolvedSourceDir);
  for (const filePath of files) {
    const rel = relative(resolvedSourceDir, filePath).replace(/\\/g, "/");
    const objectKey = `plugins/${pluginId}/${version}/${rel}`;
    const contentType = detectMimeType(filePath);

    console.log(`Uploading ${objectKey}`);
    runWrangler([
      "r2",
      "object",
      "put",
      `${bucket}/${objectKey}`,
      "--file",
      filePath,
      "--content-type",
      contentType,
    ]);
  }

  const manifestObjectKey = `plugins/${pluginId}/${version}/manifest.json`;
  console.log(`Uploading ${manifestObjectKey}`);
  runWrangler([
    "r2",
    "object",
    "put",
    `${bucket}/${manifestObjectKey}`,
    "--file",
    signedManifestPath,
    "--content-type",
    "application/json",
  ]);

  const escapedManifest = escapeSql(JSON.stringify(signedManifest));
  const escapedPluginId = escapeSql(pluginId);
  const escapedVersion = escapeSql(version);
  const escapedDisplayName = escapeSql(
    signedManifest.displayName || pluginId,
  );
  const escapedDescription = escapeSql(signedManifest.description || "");
  const escapedPanel = escapeSql(signedManifest.mount?.panel || "frames");
  const escapedChannel = escapeSql(channel);

  const sql = `
    INSERT INTO plugins (plugin_id, display_name, description, panel, status, created_at, updated_at)
    VALUES ('${escapedPluginId}', '${escapedDisplayName}', '${escapedDescription}', '${escapedPanel}', 'active', datetime('now'), datetime('now'))
    ON CONFLICT(plugin_id) DO UPDATE SET
      display_name = excluded.display_name,
      description = excluded.description,
      panel = excluded.panel,
      status = 'active',
      updated_at = datetime('now');

    UPDATE plugin_releases
    SET is_active = 0
    WHERE plugin_id = '${escapedPluginId}'
      AND channel = '${escapedChannel}';

    INSERT INTO plugin_releases (plugin_id, version, channel, manifest_json, is_active, created_at)
    VALUES ('${escapedPluginId}', '${escapedVersion}', '${escapedChannel}', '${escapedManifest}', 1, datetime('now'))
    ON CONFLICT(plugin_id, version) DO UPDATE SET
      channel = excluded.channel,
      manifest_json = excluded.manifest_json,
      is_active = 1;
  `;

  console.log("Upserting plugin metadata into D1");
  runWrangler([
    "d1",
    "execute",
    database,
    "--command",
    sql,
  ]);

  console.log(`Publish complete: ${pluginId}@${version}`);
}

main();

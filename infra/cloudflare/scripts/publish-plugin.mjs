#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { createSign } from "node:crypto";
import {
    readdirSync,
    readFileSync,
    statSync,
    writeFileSync,
    mkdirSync,
    existsSync,
} from "node:fs";
import { dirname, join, relative, resolve, extname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const DEFAULT_CHANNEL = "stable";
const DEFAULT_KEY_ID = "lumilio-prod-1";
const ARTIFACT_CACHE_CONTROL = "public, max-age=31536000, immutable";
const SIGNATURE_PLACEHOLDER_PREFIX = "REPLACE_WITH_";
const SAFE_SEGMENT_PATTERN = /^[A-Za-z0-9._+-]+$/;
const SUPPORTED_PLUGIN_IMAGE_MIME_TYPES = [
    "image/jpeg",
    "image/png",
    "image/webp",
];
const DEFAULT_PLUGIN_INPUT_MIME_TYPES = [...SUPPORTED_PLUGIN_IMAGE_MIME_TYPES];
const DEFAULT_PLUGIN_OUTPUT_MIME_TYPES = [
    "image/png",
    "image/jpeg",
    "image/webp",
];
const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
const DEFAULT_WRANGLER_CONFIG = resolve(
    SCRIPT_DIR,
    "../registry-worker/wrangler.toml",
);

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
    --bucket lumilio-plugin-artifacts

Optional:
  --channel stable
  --cdn-origin https://cdn.example.com
  --db DB
  --wrangler-config infra/cloudflare/registry-worker/wrangler.toml
  --local
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
    // Keep canonicalization aligned with web/src/features/studio/plugins/signature.ts.
    const manifestWithoutSignature = {
        schemaVersion: manifest.schemaVersion,
        id: manifest.id,
        version: manifest.version,
        displayName: manifest.displayName,
        description: manifest.description,
        mount: manifest.mount,
        entries: manifest.entries,
        io: manifest.io,
        permissions: manifest.permissions,
        compatibility: {
            studioApi: manifest.compatibility?.studioApi,
            minHostVersion: manifest.compatibility?.minHostVersion,
            maxHostVersion: manifest.compatibility?.maxHostVersion,
        },
    };
    return JSON.stringify(sortKeysDeep(manifestWithoutSignature));
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

function runWrangler(args, options) {
    const finalArgs = [...args, "--config", options.wranglerConfig];
    if (options.useRemote) {
        finalArgs.push("--remote");
    }

    execFileSync("wrangler", finalArgs, {
        stdio: "inherit",
    });
}

function escapeSql(value) {
    return String(value).replace(/'/g, "''");
}

function isNonEmptyString(value) {
    return typeof value === "string" && value.trim().length > 0;
}

function ensureSafeSegment(name, value) {
    if (!isNonEmptyString(value)) {
        usageAndExit(`${name} must be a non-empty string`);
    }

    if (!SAFE_SEGMENT_PATTERN.test(value)) {
        usageAndExit(`${name} contains unsupported characters: ${value}`);
    }
}

function normalizeOrigin(origin) {
    try {
        const url = new URL(origin);
        return url.origin;
    } catch {
        usageAndExit(`Invalid --cdn-origin: ${origin}`);
    }
}

function getFileNameFromEntry(entryUrl, fallback) {
    try {
        const pathname = new URL(entryUrl).pathname;
        const fileName = pathname.split("/").filter(Boolean).pop();
        return fileName || fallback;
    } catch {
        return fallback;
    }
}

function rewriteEntriesForCdnOrigin(entries, cdnOrigin, pluginId, version) {
    const uiFileName = getFileNameFromEntry(entries.ui, "ui.mjs");
    const runnerFileName = getFileNameFromEntry(entries.runner, "runner.mjs");
    const prefix = `${cdnOrigin}/plugins/${encodeURIComponent(pluginId)}/${encodeURIComponent(version)}`;

    return {
        ui: `${prefix}/${uiFileName}`,
        runner: `${prefix}/${runnerFileName}`,
    };
}

function looksLikePlaceholderUrl(value) {
    return typeof value === "string" && value.includes("example.com");
}

function validateUrl(value, fieldName) {
    if (!isNonEmptyString(value)) {
        usageAndExit(`Manifest ${fieldName} must be a non-empty string`);
    }

    try {
        const url = new URL(value);
        if (url.protocol !== "https:") {
            usageAndExit(`Manifest ${fieldName} must use https`);
        }
    } catch {
        usageAndExit(`Manifest ${fieldName} is not a valid URL`);
    }
}

function normalizeMimeTypeList(value, fieldName) {
    if (value === undefined) {
        return undefined;
    }

    if (
        !Array.isArray(value) ||
        !value.every((item) => typeof item === "string")
    ) {
        usageAndExit(`${fieldName} must be string[]`);
    }

    if (value.length === 0) {
        usageAndExit(`${fieldName} cannot be empty`);
    }

    const out = [];
    const seen = new Set();
    for (const item of value) {
        const mimeType = item.trim().toLowerCase();
        if (!SUPPORTED_PLUGIN_IMAGE_MIME_TYPES.includes(mimeType)) {
            usageAndExit(
                `${fieldName} contains unsupported mimeType '${item}'. Supported values: ${SUPPORTED_PLUGIN_IMAGE_MIME_TYPES.join(", ")}`,
            );
        }
        if (seen.has(mimeType)) {
            continue;
        }
        seen.add(mimeType);
        out.push(mimeType);
    }

    if (out.length === 0) {
        usageAndExit(`${fieldName} cannot be empty`);
    }

    return out;
}

function normalizeManifestIo(io) {
    if (io === undefined) {
        return {
            input: {
                mimeTypes: [...DEFAULT_PLUGIN_INPUT_MIME_TYPES],
            },
            output: {
                mimeTypes: [...DEFAULT_PLUGIN_OUTPUT_MIME_TYPES],
                preferredMimeType: DEFAULT_PLUGIN_OUTPUT_MIME_TYPES[0],
            },
        };
    }

    if (typeof io !== "object" || io === null) {
        usageAndExit("manifest.io must be an object");
    }

    const inputSection = io.input;
    if (
        inputSection !== undefined &&
        (typeof inputSection !== "object" || inputSection === null)
    ) {
        usageAndExit("manifest.io.input must be an object");
    }
    const inputMimeTypes = normalizeMimeTypeList(
        inputSection?.mimeTypes,
        "manifest.io.input.mimeTypes",
    ) || [...DEFAULT_PLUGIN_INPUT_MIME_TYPES];

    const outputSection = io.output;
    if (
        outputSection !== undefined &&
        (typeof outputSection !== "object" || outputSection === null)
    ) {
        usageAndExit("manifest.io.output must be an object");
    }
    const outputMimeTypes = normalizeMimeTypeList(
        outputSection?.mimeTypes,
        "manifest.io.output.mimeTypes",
    ) || [...DEFAULT_PLUGIN_OUTPUT_MIME_TYPES];

    let preferredMimeType = outputSection?.preferredMimeType;
    if (
        preferredMimeType !== undefined &&
        !isNonEmptyString(preferredMimeType)
    ) {
        usageAndExit(
            "manifest.io.output.preferredMimeType must be a non-empty string",
        );
    }
    preferredMimeType = (preferredMimeType || outputMimeTypes[0])
        .trim()
        .toLowerCase();

    if (!SUPPORTED_PLUGIN_IMAGE_MIME_TYPES.includes(preferredMimeType)) {
        usageAndExit(
            `manifest.io.output.preferredMimeType '${preferredMimeType}' is unsupported`,
        );
    }
    if (!outputMimeTypes.includes(preferredMimeType)) {
        usageAndExit(
            "manifest.io.output.preferredMimeType must be one of manifest.io.output.mimeTypes",
        );
    }

    return {
        input: {
            mimeTypes: inputMimeTypes,
        },
        output: {
            mimeTypes: outputMimeTypes,
            preferredMimeType,
        },
    };
}

function normalizeManifest(rawManifest, pluginId, version, cdnOrigin) {
    if (typeof rawManifest !== "object" || rawManifest === null) {
        usageAndExit("manifest must be a JSON object");
    }

    const manifest = {
        ...rawManifest,
    };

    manifest.id = pluginId;
    manifest.version = version;

    if (manifest.schemaVersion !== 1) {
        usageAndExit("manifest.schemaVersion must be 1");
    }

    if (!isNonEmptyString(manifest.displayName)) {
        usageAndExit("manifest.displayName is required");
    }

    if (typeof manifest.mount !== "object" || manifest.mount === null) {
        usageAndExit("manifest.mount is required");
    }

    if (manifest.mount.panel !== "plugins") {
        usageAndExit("manifest.mount.panel must be 'plugins'");
    }

    if (typeof manifest.entries !== "object" || manifest.entries === null) {
        usageAndExit("manifest.entries is required");
    }

    if (cdnOrigin) {
        manifest.entries = rewriteEntriesForCdnOrigin(
            manifest.entries,
            cdnOrigin,
            pluginId,
            version,
        );
    }

    validateUrl(manifest.entries.ui, "entries.ui");
    validateUrl(manifest.entries.runner, "entries.runner");

    if (
        !cdnOrigin &&
        (looksLikePlaceholderUrl(manifest.entries.ui) ||
            looksLikePlaceholderUrl(manifest.entries.runner))
    ) {
        usageAndExit(
            "Manifest entries still point to example.com. Pass --cdn-origin to rewrite runtime URLs for this release.",
        );
    }

    if (
        !Array.isArray(manifest.permissions) ||
        !manifest.permissions.every((item) => typeof item === "string")
    ) {
        usageAndExit("manifest.permissions must be string[]");
    }

    if (
        typeof manifest.compatibility !== "object" ||
        manifest.compatibility === null ||
        !isNonEmptyString(manifest.compatibility.studioApi)
    ) {
        usageAndExit("manifest.compatibility.studioApi is required");
    }

    manifest.io = normalizeManifestIo(manifest.io);

    if (typeof manifest.signature !== "object" || manifest.signature === null) {
        manifest.signature = {
            keyId: DEFAULT_KEY_ID,
            algorithm: "ECDSA_P256_SHA256",
            value: "",
        };
    }

    manifest.signature = {
        keyId: isNonEmptyString(manifest.signature.keyId)
            ? manifest.signature.keyId
            : DEFAULT_KEY_ID,
        algorithm: "ECDSA_P256_SHA256",
        value: isNonEmptyString(manifest.signature.value)
            ? manifest.signature.value
            : "",
    };

    return manifest;
}

function ensureSignedManifestIsUsable(manifest) {
    const signatureValue = manifest.signature?.value;
    if (!isNonEmptyString(signatureValue)) {
        usageAndExit(
            "Manifest signature is empty. Set PLUGIN_SIGNING_PRIVATE_KEY_PEM to sign during publish.",
        );
    }

    if (signatureValue.startsWith(SIGNATURE_PLACEHOLDER_PREFIX)) {
        usageAndExit(
            "Manifest signature is still a placeholder. Set PLUGIN_SIGNING_PRIVATE_KEY_PEM to sign during publish.",
        );
    }
}

function uploadR2Object(bucket, objectKey, filePath, contentType, options) {
    runWrangler(
        [
            "r2",
            "object",
            "put",
            `${bucket}/${objectKey}`,
            "--file",
            filePath,
            "--content-type",
            contentType,
            "--cache-control",
            ARTIFACT_CACHE_CONTROL,
        ],
        options,
    );
}

function resolveEntryFilePath(sourceDir, entryUrl) {
    const fileName = getFileNameFromEntry(entryUrl, "");
    if (!fileName) {
        usageAndExit(
            `Unable to infer artifact file from entry URL: ${entryUrl}`,
        );
    }
    return join(sourceDir, fileName);
}

function ensureEntryArtifactsExist(sourceDir, entries) {
    const uiPath = resolveEntryFilePath(sourceDir, entries.ui);
    const runnerPath = resolveEntryFilePath(sourceDir, entries.runner);

    if (!existsSync(uiPath)) {
        usageAndExit(`UI entry artifact not found in source dir: ${uiPath}`);
    }

    if (!existsSync(runnerPath)) {
        usageAndExit(
            `Runner entry artifact not found in source dir: ${runnerPath}`,
        );
    }
}

function maybeSignManifest(manifest) {
    const privateKeyPem = process.env.PLUGIN_SIGNING_PRIVATE_KEY_PEM;
    const keyId =
        process.env.PLUGIN_SIGNING_KEY_ID ||
        manifest.signature?.keyId ||
        DEFAULT_KEY_ID;

    if (!privateKeyPem) {
        return manifest;
    }

    const payload = canonicalizeManifestPayload(manifest);
    const signer = createSign("SHA256");
    signer.update(payload);
    signer.end();

    // WebCrypto verification in Studio expects raw r||s (IEEE-P1363), not DER.
    const signature = signer.sign({
        key: privateKeyPem,
        dsaEncoding: "ieee-p1363",
    });

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
    const database = args.db || "DB";
    const channel = args.channel || DEFAULT_CHANNEL;
    const cdnOrigin = args["cdn-origin"]
        ? normalizeOrigin(args["cdn-origin"])
        : null;
    const wranglerConfig = args["wrangler-config"]
        ? resolve(args["wrangler-config"])
        : DEFAULT_WRANGLER_CONFIG;
    const useRemote = args.local !== "true";

    if (!pluginId) usageAndExit("--plugin-id is required");
    if (!version) usageAndExit("--version is required");
    if (!sourceDir) usageAndExit("--source is required");
    if (!manifestPath) usageAndExit("--manifest is required");
    if (!bucket) usageAndExit("--bucket is required");
    ensureSafeSegment("plugin-id", pluginId);
    ensureSafeSegment("version", version);
    ensureSafeSegment("channel", channel);
    if (!isNonEmptyString(database)) {
        usageAndExit("--db must be a non-empty string");
    }

    const resolvedSourceDir = resolve(sourceDir);
    const resolvedManifestPath = resolve(manifestPath);
    const runOptions = {
        useRemote,
        wranglerConfig,
    };

    if (!statSync(resolvedSourceDir).isDirectory()) {
        usageAndExit(`source directory not found: ${resolvedSourceDir}`);
    }
    if (!existsSync(wranglerConfig)) {
        usageAndExit(`wrangler config not found: ${wranglerConfig}`);
    }

    const manifestInput = JSON.parse(
        readFileSync(resolvedManifestPath, "utf8"),
    );
    const manifest = normalizeManifest(
        manifestInput,
        pluginId,
        version,
        cdnOrigin,
    );

    const signedManifest = maybeSignManifest(manifest);
    ensureSignedManifestIsUsable(signedManifest);
    ensureEntryArtifactsExist(resolvedSourceDir, signedManifest.entries);
    const tmpFolder = resolve(tmpdir(), "lumilio-plugin-publish");
    mkdirSync(tmpFolder, { recursive: true });
    const signedManifestPath = join(
        tmpFolder,
        `${pluginId}-${version}-manifest.json`,
    );
    writeFileSync(signedManifestPath, JSON.stringify(signedManifest, null, 2));

    const files = listFilesRecursive(resolvedSourceDir);
    for (const filePath of files) {
        const rel = relative(resolvedSourceDir, filePath).replace(/\\/g, "/");
        const objectKey = `plugins/${pluginId}/${version}/${rel}`;
        const contentType = detectMimeType(filePath);

        console.log(`Uploading ${objectKey}`);
        uploadR2Object(bucket, objectKey, filePath, contentType, runOptions);
    }

    const manifestObjectKey = `plugins/${pluginId}/${version}/manifest.json`;
    console.log(`Uploading ${manifestObjectKey}`);
    uploadR2Object(
        bucket,
        manifestObjectKey,
        signedManifestPath,
        "application/json",
        runOptions,
    );

    const escapedManifest = escapeSql(JSON.stringify(signedManifest));
    const escapedPluginId = escapeSql(pluginId);
    const escapedVersion = escapeSql(version);
    const escapedDisplayName = escapeSql(
        signedManifest.displayName || pluginId,
    );
    const escapedDescription = escapeSql(signedManifest.description || "");
    const escapedPanel = escapeSql(signedManifest.mount?.panel || "plugins");
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
    runWrangler(["d1", "execute", database, "--command", sql], runOptions);

    console.log(`Publish complete: ${pluginId}@${version}`);
}

main();

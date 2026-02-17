import type { RuntimeManifestV1, StudioPluginPanel } from "./types";

const SUPPORTED_SCHEMA_VERSION = 1;
const SUPPORTED_STUDIO_API = "1";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isUrlAllowed(urlText: string, allowOrigin?: string): boolean {
  try {
    const url = new URL(urlText);
    const isSecure = url.protocol === "https:";
    const isLocal =
      url.hostname === "localhost" ||
      url.hostname === "127.0.0.1" ||
      url.hostname === "[::1]";

    if (!isSecure && !isLocal) {
      return false;
    }

    if (allowOrigin && url.origin !== allowOrigin) {
      return false;
    }

    return true;
  } catch {
    return false;
  }
}

export interface ManifestGuardOptions {
  expectedPanel?: StudioPluginPanel;
  allowOrigin?: string;
}

export function validateRuntimeManifest(
  input: unknown,
  options: ManifestGuardOptions = {},
): RuntimeManifestV1 {
  if (!isRecord(input)) {
    throw new Error("Runtime manifest is not an object");
  }

  if (input.schemaVersion !== SUPPORTED_SCHEMA_VERSION) {
    throw new Error(`Unsupported schemaVersion: ${String(input.schemaVersion)}`);
  }

  if (!isNonEmptyString(input.id)) {
    throw new Error("Manifest field 'id' is required");
  }
  const id = input.id;

  if (!isNonEmptyString(input.version)) {
    throw new Error("Manifest field 'version' is required");
  }
  const version = input.version;

  if (!isNonEmptyString(input.displayName)) {
    throw new Error("Manifest field 'displayName' is required");
  }
  const displayName = input.displayName;

  if (!isRecord(input.mount)) {
    throw new Error("Manifest field 'mount' is required");
  }

  const panel = input.mount.panel;
  if (panel !== "frames" && panel !== "develop") {
    throw new Error("Manifest mount.panel must be 'frames' or 'develop'");
  }

  if (options.expectedPanel && panel !== options.expectedPanel) {
    throw new Error(
      `Manifest panel mismatch: expected ${options.expectedPanel}, got ${String(panel)}`,
    );
  }
  const mountOrder =
    typeof input.mount.order === "number" && Number.isFinite(input.mount.order)
      ? input.mount.order
      : undefined;

  if (!isRecord(input.entries)) {
    throw new Error("Manifest field 'entries' is required");
  }

  if (!isNonEmptyString(input.entries.ui) || !isUrlAllowed(input.entries.ui, options.allowOrigin)) {
    throw new Error("Manifest entries.ui is invalid or not allowed");
  }
  const entryUi = input.entries.ui;

  if (
    !isNonEmptyString(input.entries.runner) ||
    !isUrlAllowed(input.entries.runner, options.allowOrigin)
  ) {
    throw new Error("Manifest entries.runner is invalid or not allowed");
  }
  const entryRunner = input.entries.runner;

  if (!isRecord(input.compatibility)) {
    throw new Error("Manifest field 'compatibility' is required");
  }

  if (!isNonEmptyString(input.compatibility.studioApi)) {
    throw new Error("Manifest compatibility.studioApi is required");
  }
  const studioApi = input.compatibility.studioApi;

  if (studioApi !== SUPPORTED_STUDIO_API) {
    throw new Error(
      `Incompatible studioApi '${studioApi}', expected '${SUPPORTED_STUDIO_API}'`,
    );
  }

  if (!isStringArray(input.permissions)) {
    throw new Error("Manifest field 'permissions' must be string[]");
  }
  const permissions = [...input.permissions];

  if (!isRecord(input.signature)) {
    throw new Error("Manifest field 'signature' is required");
  }

  if (!isNonEmptyString(input.signature.keyId)) {
    throw new Error("Manifest signature.keyId is required");
  }
  const signatureKeyId = input.signature.keyId;

  if (input.signature.algorithm !== "ECDSA_P256_SHA256") {
    throw new Error(
      `Unsupported signature algorithm: ${String(input.signature.algorithm)}`,
    );
  }

  if (!isNonEmptyString(input.signature.value)) {
    throw new Error("Manifest signature.value is required");
  }
  const signatureValue = input.signature.value;

  const description = isNonEmptyString(input.description) ? input.description : undefined;
  const minHostVersion = isNonEmptyString(input.compatibility.minHostVersion)
    ? input.compatibility.minHostVersion
    : undefined;
  const maxHostVersion = isNonEmptyString(input.compatibility.maxHostVersion)
    ? input.compatibility.maxHostVersion
    : undefined;

  return {
    schemaVersion: SUPPORTED_SCHEMA_VERSION,
    id,
    version,
    displayName,
    description,
    mount: {
      panel,
      order: mountOrder,
    },
    entries: {
      ui: entryUi,
      runner: entryRunner,
    },
    permissions,
    compatibility: {
      studioApi,
      minHostVersion,
      maxHostVersion,
    },
    signature: {
      keyId: signatureKeyId,
      algorithm: "ECDSA_P256_SHA256",
      value: signatureValue,
    },
  };
}

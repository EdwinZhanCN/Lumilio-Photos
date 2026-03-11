import {
  STUDIO_PLUGIN_IMAGE_MIME_TYPES,
  type RuntimeManifestIo,
  type RuntimeManifestIoInput,
  type RuntimeManifestIoOutput,
  type RuntimeManifestV1,
  type StudioPluginImageMimeType,
  type StudioPluginPanel,
} from "./types";

const SUPPORTED_SCHEMA_VERSION = 1;
const SUPPORTED_STUDIO_API = "1";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function isStringArray(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === "string")
  );
}

const STUDIO_PLUGIN_IMAGE_MIME_TYPE_SET = new Set<string>(
  STUDIO_PLUGIN_IMAGE_MIME_TYPES,
);

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

function normalizeManifestMimeTypes(
  value: unknown,
  fieldPath: string,
): StudioPluginImageMimeType[] | undefined {
  if (value === undefined) {
    return undefined;
  }

  if (!isStringArray(value)) {
    throw new Error(`${fieldPath} must be string[]`);
  }

  if (value.length === 0) {
    throw new Error(`${fieldPath} cannot be empty`);
  }

  const seen = new Set<string>();
  const out: StudioPluginImageMimeType[] = [];
  for (const raw of value) {
    const mimeType = raw.trim().toLowerCase();
    if (!STUDIO_PLUGIN_IMAGE_MIME_TYPE_SET.has(mimeType)) {
      throw new Error(
        `${fieldPath} contains unsupported mime type '${raw}'. Supported: ${STUDIO_PLUGIN_IMAGE_MIME_TYPES.join(", ")}`,
      );
    }
    if (seen.has(mimeType)) {
      continue;
    }
    seen.add(mimeType);
    out.push(mimeType as StudioPluginImageMimeType);
  }

  if (out.length === 0) {
    throw new Error(`${fieldPath} cannot be empty`);
  }

  return out;
}

function normalizeManifestIo(input: unknown): RuntimeManifestIo | undefined {
  if (input === undefined) {
    return undefined;
  }

  if (!isRecord(input)) {
    throw new Error("Manifest field 'io' must be an object");
  }

  let ioInput: RuntimeManifestIoInput | undefined;
  if (input.input !== undefined) {
    if (!isRecord(input.input)) {
      throw new Error("Manifest field 'io.input' must be an object");
    }

    const mimeTypes = normalizeManifestMimeTypes(
      input.input.mimeTypes,
      "Manifest io.input.mimeTypes",
    );

    ioInput = mimeTypes ? { mimeTypes } : {};
  }

  let ioOutput: RuntimeManifestIoOutput | undefined;
  if (input.output !== undefined) {
    if (!isRecord(input.output)) {
      throw new Error("Manifest field 'io.output' must be an object");
    }

    const mimeTypes = normalizeManifestMimeTypes(
      input.output.mimeTypes,
      "Manifest io.output.mimeTypes",
    );

    if (
      input.output.preferredMimeType !== undefined &&
      !isNonEmptyString(input.output.preferredMimeType)
    ) {
      throw new Error(
        "Manifest io.output.preferredMimeType must be a non-empty string",
      );
    }

    const preferredMimeTypeRaw = input.output.preferredMimeType
      ?.trim()
      .toLowerCase();
    if (
      preferredMimeTypeRaw !== undefined &&
      !STUDIO_PLUGIN_IMAGE_MIME_TYPE_SET.has(preferredMimeTypeRaw)
    ) {
      throw new Error(
        `Manifest io.output.preferredMimeType '${input.output.preferredMimeType}' is unsupported`,
      );
    }

    if (
      preferredMimeTypeRaw !== undefined &&
      mimeTypes &&
      !mimeTypes.includes(preferredMimeTypeRaw as StudioPluginImageMimeType)
    ) {
      throw new Error(
        "Manifest io.output.preferredMimeType must be one of io.output.mimeTypes",
      );
    }

    ioOutput = {
      ...(mimeTypes ? { mimeTypes } : {}),
      ...(preferredMimeTypeRaw
        ? {
            preferredMimeType:
              preferredMimeTypeRaw as StudioPluginImageMimeType,
          }
        : {}),
    };
  }

  const io: RuntimeManifestIo = {
    ...(ioInput ? { input: ioInput } : {}),
    ...(ioOutput ? { output: ioOutput } : {}),
  };

  return io;
}

export function validateRuntimeManifest(
  input: unknown,
  options: ManifestGuardOptions = {},
): RuntimeManifestV1 {
  if (!isRecord(input)) {
    throw new Error("Runtime manifest is not an object");
  }

  if (input.schemaVersion !== SUPPORTED_SCHEMA_VERSION) {
    throw new Error(
      `Unsupported schemaVersion: ${String(input.schemaVersion)}`,
    );
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
  if (panel !== "plugins") {
    throw new Error("Manifest mount.panel must be 'plugins'");
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

  if (
    !isNonEmptyString(input.entries.ui) ||
    !isUrlAllowed(input.entries.ui, options.allowOrigin)
  ) {
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
  const io = normalizeManifestIo(input.io);

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

  const description = isNonEmptyString(input.description)
    ? input.description
    : undefined;
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
    ...(io ? { io } : {}),
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

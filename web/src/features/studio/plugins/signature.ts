import { STUDIO_PLUGIN_KEY_RING, type StudioPluginKeyRing } from "./keyring";
import type { RuntimeManifestV1 } from "./types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function sortKeysDeep(input: unknown): unknown {
  if (Array.isArray(input)) {
    return input.map(sortKeysDeep);
  }

  if (!isRecord(input)) {
    return input;
  }

  const sortedKeys = Object.keys(input).sort();
  const out: Record<string, unknown> = {};
  for (const key of sortedKeys) {
    out[key] = sortKeysDeep(input[key]);
  }
  return out;
}

function base64ToBytes(base64: string): Uint8Array {
  const normalized = base64.replace(/-/g, "+").replace(/_/g, "/");
  const pad = normalized.length % 4;
  const padded = pad === 0 ? normalized : normalized + "=".repeat(4 - pad);
  const binary = atob(padded);
  const out = new Uint8Array(binary.length);

  for (let i = 0; i < binary.length; i += 1) {
    out[i] = binary.charCodeAt(i);
  }

  return out;
}

function toArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  // Create a detached copy to satisfy strict BufferSource typing.
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}

export function canonicalizeManifestPayload(manifest: RuntimeManifestV1): string {
  const manifestWithoutSignature: Omit<RuntimeManifestV1, "signature"> = {
    schemaVersion: manifest.schemaVersion,
    id: manifest.id,
    version: manifest.version,
    displayName: manifest.displayName,
    description: manifest.description,
    mount: manifest.mount,
    entries: manifest.entries,
    permissions: manifest.permissions,
    compatibility: manifest.compatibility,
  };

  return JSON.stringify(sortKeysDeep(manifestWithoutSignature));
}

async function importSpkiPublicKey(spkiBase64: string): Promise<CryptoKey> {
  const keyData = base64ToBytes(spkiBase64);
  return crypto.subtle.importKey(
    "spki",
    toArrayBuffer(keyData),
    {
      name: "ECDSA",
      namedCurve: "P-256",
    },
    false,
    ["verify"],
  );
}

export async function verifyRuntimeManifestSignature(
  manifest: RuntimeManifestV1,
  keyRing: StudioPluginKeyRing = STUDIO_PLUGIN_KEY_RING,
): Promise<boolean> {
  if (!crypto?.subtle) {
    return false;
  }

  const keyBase64 = keyRing[manifest.signature.keyId];
  if (!keyBase64) {
    return false;
  }

  try {
    const key = await importSpkiPublicKey(keyBase64);
    const signatureBytes = base64ToBytes(manifest.signature.value);
    const payload = new TextEncoder().encode(canonicalizeManifestPayload(manifest));

    return crypto.subtle.verify(
      {
        name: "ECDSA",
        hash: "SHA-256",
      },
      key,
      toArrayBuffer(signatureBytes),
      toArrayBuffer(payload),
    );
  } catch {
    return false;
  }
}

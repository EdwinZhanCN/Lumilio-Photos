import { describe, expect, it } from "vitest";
import {
  canonicalizeManifestPayload,
  verifyRuntimeManifestSignature,
} from "./signature";
import type { RuntimeManifestV1 } from "./types";

function bytesToBase64Url(bytes: Uint8Array): string {
  const binary = String.fromCharCode(...bytes);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function bytesToBase64(bytes: Uint8Array): string {
  const binary = String.fromCharCode(...bytes);
  return btoa(binary);
}

function createUnsignedManifest(): RuntimeManifestV1 {
  return {
    schemaVersion: 1,
    id: "com.lumilio.border",
    version: "0.1.0",
    displayName: "Lumilio Border",
    mount: {
      panel: "frames",
      order: 10,
    },
    entries: {
      ui: "https://cdn.example.com/plugins/com.lumilio.border/0.1.0/ui.mjs",
      runner: "https://cdn.example.com/plugins/com.lumilio.border/0.1.0/runner.mjs",
    },
    permissions: ["image.read", "image.write"],
    compatibility: {
      studioApi: "1",
    },
    signature: {
      keyId: "test-key",
      algorithm: "ECDSA_P256_SHA256",
      value: "",
    },
  };
}

describe("verifyRuntimeManifestSignature", () => {
  it("verifies a valid signature", async () => {
    const keyPair = await crypto.subtle.generateKey(
      {
        name: "ECDSA",
        namedCurve: "P-256",
      },
      true,
      ["sign", "verify"],
    );

    const publicSpki = await crypto.subtle.exportKey("spki", keyPair.publicKey);
    const publicSpkiBase64 = bytesToBase64(new Uint8Array(publicSpki));

    const manifest = createUnsignedManifest();
    const payload = new TextEncoder().encode(canonicalizeManifestPayload(manifest));

    const signatureBuffer = await crypto.subtle.sign(
      {
        name: "ECDSA",
        hash: "SHA-256",
      },
      keyPair.privateKey,
      payload,
    );

    manifest.signature.value = bytesToBase64Url(new Uint8Array(signatureBuffer));

    const ok = await verifyRuntimeManifestSignature(manifest, {
      "test-key": publicSpkiBase64,
    });

    expect(ok).toBe(true);
  });

  it("fails when signature is tampered", async () => {
    const manifest = createUnsignedManifest();
    manifest.signature.value = "dGFtcGVyZWQ";

    const ok = await verifyRuntimeManifestSignature(manifest, {
      "test-key": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEU4eRYQnB+vMSlySsDzaORNOvQyHZYwj9tnjBfa9mIly5JnE16aTSpwzRU/7kiHyhcHdXJrsydD2u3IGUxcN5zw==",
    });

    expect(ok).toBe(false);
  });
});

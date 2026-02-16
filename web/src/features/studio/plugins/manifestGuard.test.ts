import { describe, expect, it } from "vitest";
import { validateRuntimeManifest } from "./manifestGuard";
import type { RuntimeManifestV1 } from "./types";

function createManifest(): RuntimeManifestV1 {
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
      keyId: "lumilio-dev-1",
      algorithm: "ECDSA_P256_SHA256",
      value: "dGVzdA==",
    },
  };
}

describe("validateRuntimeManifest", () => {
  it("accepts valid manifest", () => {
    const manifest = createManifest();
    const output = validateRuntimeManifest(manifest, {
      allowOrigin: "https://cdn.example.com",
      expectedPanel: "frames",
    });

    expect(output.id).toBe("com.lumilio.border");
  });

  it("rejects wrong panel", () => {
    const manifest = createManifest();
    manifest.mount.panel = "develop";

    expect(() =>
      validateRuntimeManifest(manifest, {
        expectedPanel: "frames",
      }),
    ).toThrow(/panel mismatch/i);
  });

  it("rejects disallowed origin", () => {
    const manifest = createManifest();

    expect(() =>
      validateRuntimeManifest(manifest, {
        allowOrigin: "https://another.example.com",
      }),
    ).toThrow(/not allowed/i);
  });
});

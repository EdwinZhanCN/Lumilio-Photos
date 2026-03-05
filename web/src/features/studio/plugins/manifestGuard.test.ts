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
      panel: "plugins",
      order: 10,
    },
    entries: {
      ui: "https://cdn.example.com/plugins/com.lumilio.border/0.1.0/ui.mjs",
      runner:
        "https://cdn.example.com/plugins/com.lumilio.border/0.1.0/runner.mjs",
    },
    io: {
      input: {
        mimeTypes: ["image/jpeg", "image/png", "image/webp"],
      },
      output: {
        mimeTypes: ["image/png"],
        preferredMimeType: "image/png",
      },
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
      expectedPanel: "plugins",
    });

    expect(output.id).toBe("com.lumilio.border");
  });

  it("rejects wrong panel", () => {
    const manifest = createManifest();
    manifest.mount.panel =
      "frames" as unknown as RuntimeManifestV1["mount"]["panel"];

    expect(() =>
      validateRuntimeManifest(manifest, {
        expectedPanel: "plugins",
      }),
    ).toThrow(/must be 'plugins'/i);
  });

  it("rejects disallowed origin", () => {
    const manifest = createManifest();

    expect(() =>
      validateRuntimeManifest(manifest, {
        allowOrigin: "https://another.example.com",
      }),
    ).toThrow(/not allowed/i);
  });

  it("rejects unsupported io input mimeTypes", () => {
    const manifest = createManifest();
    manifest.io = {
      input: {
        mimeTypes: ["image/heic"] as any,
      },
    };

    expect(() => validateRuntimeManifest(manifest)).toThrow(
      /unsupported mime type/i,
    );
  });

  it("rejects preferred output mimeType not present in output mimeTypes", () => {
    const manifest = createManifest();
    manifest.io = {
      output: {
        mimeTypes: ["image/png"],
        preferredMimeType: "image/webp",
      },
    };

    expect(() => validateRuntimeManifest(manifest)).toThrow(
      /preferredMimeType must be one of io\.output\.mimeTypes/i,
    );
  });
});

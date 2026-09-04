import { describe, expect, it } from "vite-plus/test";
import { getCurrentLocationCapability } from "./locationCapability";

describe("getCurrentLocationCapability", () => {
  it("requires a secure context before exposing current location", () => {
    expect(getCurrentLocationCapability({ secureContext: false, geolocationAvailable: true })).toBe(
      "secure-context-required",
    );
  });

  it("reports browsers without geolocation support", () => {
    expect(getCurrentLocationCapability({ secureContext: true, geolocationAvailable: false })).toBe(
      "unsupported",
    );
  });

  it("allows geolocation only when both requirements are satisfied", () => {
    expect(getCurrentLocationCapability({ secureContext: true, geolocationAvailable: true })).toBe(
      "available",
    );
  });
});

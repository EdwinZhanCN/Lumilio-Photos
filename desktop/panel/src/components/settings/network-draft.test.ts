import { describe, expect, it } from "vite-plus/test";
import type { RuntimeNetworkSummary } from "../../lib/types.ts";
import { networkDraftFromResolvedState, proxyLocationFor } from "./network-draft.ts";

const external: RuntimeNetworkSummary = {
  mode: "external_https",
  listen: "0.0.0.0:6680",
  primaryOrigin: "https://photos.example.com",
  tlsMode: "external",
  proxyMode: "required",
  trustedProxyCIDRs: ["192.168.1.10/32"],
  passkeyOrigin: "https://photos.example.com",
  rpID: "photos.example.com",
  passkeyEnabled: true,
  remotePasskeyAvailable: true,
};

describe("structured network draft", () => {
  it("combines strict runtime facts with host-only LAN facts", () => {
    const draft = networkDraftFromResolvedState(external, {
      lanWarningAcceptedVersion: 1,
      lanAddresses: ["192.168.1.42"],
    });
    expect(draft).toEqual({
      mode: "external_https",
      primaryOrigin: "https://photos.example.com",
      listen: "0.0.0.0:6680",
      trustedProxyCIDRs: ["192.168.1.10/32"],
      lanWarningAcceptedVersion: 1,
      lanAddresses: ["192.168.1.42"],
    });
    expect(proxyLocationFor(draft)).toBe("remote");
  });

  it("recognizes the loopback-only same-host proxy profile", () => {
    expect(
      proxyLocationFor({
        ...networkDraftFromResolvedState(external, {
          lanWarningAcceptedVersion: 0,
          lanAddresses: [],
        }),
        trustedProxyCIDRs: ["127.0.0.1/32", "::1/128"],
      }),
    ).toBe("same_host");
  });
});

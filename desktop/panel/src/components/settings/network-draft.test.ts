import { describe, expect, it } from "vite-plus/test";
import type { RuntimeNetworkSummary } from "../../lib/types.ts";
import { networkDraftFromResolvedState } from "./network-draft.ts";

const lan: RuntimeNetworkSummary = {
  mode: "lan_http",
  listen: "0.0.0.0:6680",
  passkeyEnabled: true,
};

describe("structured network draft", () => {
  it("combines runtime mode with host-only LAN facts", () => {
    expect(
      networkDraftFromResolvedState(lan, {
        lanWarningAcceptedVersion: 1,
        lanAddresses: ["192.168.1.42"],
      }),
    ).toEqual({
      mode: "lan_http",
      listen: "0.0.0.0:6680",
      lanWarningAcceptedVersion: 1,
      lanAddresses: ["192.168.1.42"],
    });
  });
});

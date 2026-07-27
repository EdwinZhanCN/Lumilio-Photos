import type { NetworkInfo, PanelState, RuntimeNetworkSummary } from "../../lib/types.ts";

export function networkDraftFromResolvedState(
  runtime: RuntimeNetworkSummary,
  host: PanelState["networkHost"],
): NetworkInfo {
  return {
    mode: runtime.mode,
    primaryOrigin: runtime.primaryOrigin,
    listen: runtime.listen,
    trustedProxyCIDRs: [...runtime.trustedProxyCIDRs],
    lanWarningAcceptedVersion: host.lanWarningAcceptedVersion,
    lanAddresses: [...host.lanAddresses],
  };
}

export function proxyLocationFor(network: NetworkInfo): "same_host" | "remote" {
  return network.trustedProxyCIDRs.some((cidr) => cidr !== "127.0.0.1/32" && cidr !== "::1/128")
    ? "remote"
    : "same_host";
}

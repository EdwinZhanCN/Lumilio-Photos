import type { NetworkInfo, PanelState, RuntimeNetworkSummary } from "../../lib/types.ts";

export function networkDraftFromResolvedState(
  runtime: RuntimeNetworkSummary,
  host: PanelState["networkHost"],
): NetworkInfo {
  return {
    mode: runtime.mode,
    listen: runtime.listen,
    lanWarningAcceptedVersion: host.lanWarningAcceptedVersion,
    lanAddresses: [...host.lanAddresses],
  };
}

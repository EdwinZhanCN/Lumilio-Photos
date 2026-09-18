import { describe, expect, it } from "vite-plus/test";
import { translateDouble } from "@test/translateDouble";
import type { StorageDiagnostic } from "@/features/repositories";
import {
  capacityUsedPercent,
  reachabilityLabel,
  riskLabel,
  storageItemSeverity,
  storageRiskKeys,
} from "./storageSeverity";

/**
 * Only diagnostic fields vary per case; identity stays fixed so a fixture cannot
 * accidentally describe a Repository while claiming to be a Storage Location.
 */
type DiagnosticOverrides = Omit<StorageDiagnostic, "entityType" | "rawName" | "path">;

function target(overrides: DiagnosticOverrides = {}): StorageDiagnostic {
  return {
    entityType: "storage_location",
    kind: "external",
    rawName: "Archive Disk",
    path: "/archive",
    reachability: "active",
    writable: true,
    ...overrides,
  };
}

describe("storageItemSeverity", () => {
  it("fails a target that cannot be reached or identified", () => {
    for (const reachability of ["offline", "identity_error", "recovery_required"]) {
      expect(storageItemSeverity(target({ reachability })), reachability).toBe("error");
    }
  });

  it("warns about a reachable target that cannot be written", () => {
    expect(storageItemSeverity(target({ writable: false }))).toBe("warning");
  });

  it("warns about any recorded risk even when the target is writable", () => {
    expect(storageItemSeverity(target({ risk_warnings: ["network_filesystem"] }))).toBe("warning");
  });

  it("passes a reachable writable target with no risk", () => {
    expect(storageItemSeverity(target())).toBe("healthy");
  });
});

describe("capacityUsedPercent", () => {
  it("reports 0 while capacity is unknown or degenerate", () => {
    expect(capacityUsedPercent(target({ capacity_known: false }))).toBe(0);
    expect(capacityUsedPercent(target({ capacity_known: true, total_bytes: 0 }))).toBe(0);
    expect(capacityUsedPercent(target({ capacity_known: true }))).toBe(0);
  });

  it("computes used share from total and available", () => {
    expect(
      capacityUsedPercent(
        target({ capacity_known: true, total_bytes: 1_000_000, available_bytes: 600_000 }),
      ),
    ).toBe(40);
  });

  it("clamps a nonsense available value instead of rendering a negative share", () => {
    expect(
      capacityUsedPercent(
        target({ capacity_known: true, total_bytes: 1_000, available_bytes: -500 }),
      ),
    ).toBe(100);
    expect(
      capacityUsedPercent(
        target({ capacity_known: true, total_bytes: 1_000, available_bytes: 5_000 }),
      ),
    ).toBe(0);
  });
});

describe("storageRiskKeys", () => {
  it("derives risks from flags as well as recorded warnings", () => {
    expect(
      storageRiskKeys(
        target({
          risk_warnings: [],
          removable_likely: true,
          network_filesystem: true,
          cloud_sync_provider: "dropbox",
          mount_fingerprint_changed: true,
        }),
      ).sort(),
    ).toEqual([
      "cloud_sync_directory",
      "mount_fingerprint_changed",
      "network_filesystem",
      "removable_storage",
    ]);
  });

  it("does not double-report a risk that is both recorded and flagged", () => {
    expect(
      storageRiskKeys(target({ risk_warnings: ["removable_storage"], removable_likely: true })),
    ).toEqual(["removable_storage"]);
  });

  it("reports nothing for an unremarkable target", () => {
    expect(storageRiskKeys(target())).toEqual([]);
  });
});

describe("storage labels", () => {
  it("resolves reachability to its own key and passes unknown words through", () => {
    const { t } = translateDouble();
    expect(reachabilityLabel(t, "active")).toBe("monitor.storage.statusActive");
    expect(reachabilityLabel(t, "recovery_required")).toBe(
      "monitor.storage.statusRecoveryRequired",
    );
    expect(reachabilityLabel(t, "future_state")).toBe("future_state");
    expect(reachabilityLabel(t, undefined)).toBe("monitor.storage.statusUnknown");
  });

  it("resolves known risks and passes unknown ones through", () => {
    const { t } = translateDouble();
    expect(riskLabel(t, "network_filesystem")).toBe("monitor.storage.riskNetwork");
    expect(riskLabel(t, "future_risk")).toBe("future_risk");
  });
});

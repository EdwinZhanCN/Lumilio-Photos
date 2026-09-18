import type { TFunction } from "i18next";
import type { StorageDiagnostic } from "@/features/repositories";

/**
 * Storage diagnostic rules.
 *
 * Reachability, writability, capacity, and risk classification decide how a
 * Storage Location or Repository is presented. They live here rather than in a
 * component so the same subject cannot be graded differently in the tree, the
 * detail pane, and the toolbar. Storage *identity* (display names, `kind`,
 * `role`) stays owned by the Repositories feature.
 */

export type StorageSeverity = "healthy" | "warning" | "error";

/** A target is healthy only when it is reachable and not known read-only. */
export function isStorageHealthy(item: StorageDiagnostic): boolean {
  return item.reachability === "active" && item.writable !== false;
}

/**
 * Operator-facing severity. Unreachable and ambiguous identities are errors;
 * degraded health or any recorded risk is a warning.
 */
export function storageItemSeverity(item: StorageDiagnostic): StorageSeverity {
  if (
    item.reachability === "offline" ||
    item.reachability === "identity_error" ||
    item.reachability === "recovery_required"
  ) {
    return "error";
  }
  if (!isStorageHealthy(item) || (item.risk_warnings ?? []).length > 0) {
    return "warning";
  }
  return "healthy";
}

/** Used capacity as a 0-100 percentage; 0 when capacity is unknown. */
export function capacityUsedPercent(item: StorageDiagnostic): number {
  if (!item.capacity_known || !item.total_bytes || item.total_bytes <= 0) return 0;
  const available = Math.max(0, item.available_bytes ?? 0);
  return Math.min(100, Math.max(0, ((item.total_bytes - available) / item.total_bytes) * 100));
}

/**
 * Every risk that applies to a target. Recorded warnings are unioned with the
 * facts that imply one, so a target cannot look safe because a flag was not
 * repeated in `risk_warnings`.
 */
export function storageRiskKeys(item: StorageDiagnostic): string[] {
  const risks = new Set(item.risk_warnings ?? []);
  if (item.removable_likely) risks.add("removable_storage");
  if (item.network_filesystem) risks.add("network_filesystem");
  if (item.cloud_sync_provider) risks.add("cloud_sync_directory");
  if (item.mount_fingerprint_changed) risks.add("mount_fingerprint_changed");
  return [...risks];
}

export function reachabilityLabel(t: TFunction, value: string | undefined): string {
  switch (value) {
    case "active":
      return t("monitor.storage.statusActive", "Available");
    case "maintenance":
      return t("monitor.storage.statusMaintenance", "Maintenance");
    case "offline":
      return t("monitor.storage.statusOffline", "Offline");
    case "identity_error":
      return t("monitor.storage.statusIdentityError", "Identity mismatch");
    case "recovery_required":
      return t("monitor.storage.statusRecoveryRequired", "Recovery required");
    default:
      return value || t("monitor.storage.statusUnknown", "Unknown");
  }
}

export function riskLabel(t: TFunction, risk: string): string {
  const labels: Record<string, string> = {
    removable_storage: t("monitor.storage.riskRemovable", "Removable storage"),
    network_filesystem: t("monitor.storage.riskNetwork", "Network filesystem"),
    cloud_sync_directory: t("monitor.storage.riskCloudSync", "Cloud-sync directory"),
    mount_fingerprint_changed: t("monitor.storage.riskMountChanged", "Mount identity changed"),
  };
  return labels[risk] ?? risk;
}

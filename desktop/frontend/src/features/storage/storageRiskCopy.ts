import type { TFunction } from "i18next";

/**
 * The Desktop's copy for the Server's closed storage risk vocabulary, one
 * literal entry per code the shared classifier can report on a native task.
 * The architecture check compares these keys with the Server's constants, so
 * a new code lands with its copy here as well as in the Web.
 */
export type StorageRiskCode =
  | "removable_storage"
  | "non_default_location"
  | "cloud_sync_directory"
  | "network_filesystem"
  | "mount_fingerprint_changed";

const RISK_LABELS: Record<StorageRiskCode, (t: TFunction) => string> = {
  removable_storage: (t) => t("storage.riskRemovable", "Removable storage"),
  non_default_location: (t) => t("storage.riskNonDefault", "Outside the Default Storage Location"),
  cloud_sync_directory: (t) => t("storage.riskCloudSync", "Cloud-sync managed folder"),
  network_filesystem: (t) => t("storage.riskNetwork", "Network filesystem"),
  mount_fingerprint_changed: (t) => t("storage.riskMountChanged", "Mounted filesystem changed"),
};

function isStorageRiskCode(code: string): code is StorageRiskCode {
  return Object.hasOwn(RISK_LABELS, code);
}

/** Localized label for one risk code; an unknown code is shown as its identifier. */
export function storageRiskLabel(code: string, t: TFunction): string {
  return isStorageRiskCode(code) ? RISK_LABELS[code](t) : code;
}

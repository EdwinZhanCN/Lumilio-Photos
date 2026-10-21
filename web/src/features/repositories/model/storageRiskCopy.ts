import type { TFunction } from "i18next";

/**
 * Localized label per storage risk code the Server reports in a create
 * assessment, a `storage/confirmation-required` Problem, a candidate, or a
 * native host action. An explicit record with literal `t` keys, so
 * `i18next-cli extract` keeps every entry and a new Server code fails the
 * type check here instead of rendering as a raw identifier.
 */
export type StorageRiskCode =
  | "removable_storage"
  | "non_default_location"
  | "cloud_sync_directory"
  | "network_filesystem"
  | "mount_fingerprint_changed"
  | "unavailable_cloud_placeholder";

const RISK_LABELS: Record<StorageRiskCode, (t: TFunction) => string> = {
  removable_storage: (t) =>
    t(
      "manage.repositories.risks.removableStorage",
      "Removable or external disk: originals disappear while it is unplugged.",
    ),
  non_default_location: (t) =>
    t(
      "manage.repositories.risks.nonDefaultLocation",
      "Outside the Default Storage Location: this Storage Location must stay authorized and connected.",
    ),
  cloud_sync_directory: (t) =>
    t(
      "manage.repositories.risks.cloudSyncDirectory",
      "Managed by cloud-sync software: sync clients may evict originals or duplicate files.",
    ),
  network_filesystem: (t) =>
    t("manage.repositories.risks.networkFilesystem", "Network filesystem: not supported."),
  mount_fingerprint_changed: (t) =>
    t(
      "manage.repositories.risks.mountChanged",
      "The mounted filesystem changed since this location was registered.",
    ),
  unavailable_cloud_placeholder: (t) =>
    t(
      "manage.repositories.risks.cloudPlaceholder",
      "Some files are cloud placeholders that must be downloaded first.",
    ),
};

function isStorageRiskCode(code: string): code is StorageRiskCode {
  return Object.hasOwn(RISK_LABELS, code);
}

/** Localized label for one risk code; an unknown code is shown as its identifier. */
export function storageRiskLabel(code: string, t: TFunction): string {
  return isStorageRiskCode(code) ? RISK_LABELS[code](t) : code;
}

import type { TFunction } from "i18next";

/**
 * Labels for the closed action vocabularies the Server's role-aware policies
 * emit for Repositories (verify, rename, detach, reconnect) and Storage
 * Locations (reconnect, detach). One literal entry per code, so the
 * architecture check can compare them with the Server's constants and a new
 * action lands with its copy.
 */
export type StorageActionCode = "verify" | "rename" | "detach" | "reconnect";

const ACTION_LABELS: Record<StorageActionCode, (t: TFunction) => string> = {
  verify: (t) => t("storagePanel.actionName.verify", "Scan"),
  rename: (t) => t("storagePanel.actionName.rename", "Rename"),
  detach: (t) => t("storagePanel.actionName.detach", "Remove from Lumilio"),
  reconnect: (t) => t("storagePanel.actionName.reconnect", "Reconnect"),
};

function isStorageActionCode(code: string): code is StorageActionCode {
  return Object.hasOwn(ACTION_LABELS, code);
}

/** Localized name of one action code; an unknown code is shown as its identifier. */
export function storageActionLabel(code: string, t: TFunction): string {
  return isStorageActionCode(code) ? ACTION_LABELS[code](t) : code;
}

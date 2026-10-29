import type { TFunction } from "i18next";

export type RecoveryItemClass = "trash" | "studio" | "staging" | "logs" | "derived" | "unknown";
export type RecoveryDisposition =
  | "preserved"
  | "moved"
  | "reassociated"
  | "ambiguous"
  | "unreadable"
  | "deleted";

/**
 * Labels for the closed recovery vocabularies. Explicit records, each entry
 * calling `t` with a literal key so `i18next-cli extract` keeps the key and
 * a missing entry fails the type check instead of showing a raw enum.
 */
const CLASS_LABELS: Record<RecoveryItemClass, (t: TFunction) => string> = {
  trash: (t) => t("storagePanel.recovery.class.trash", "Trash"),
  studio: (t) => t("storagePanel.recovery.class.studio", "Studio edit"),
  staging: (t) => t("storagePanel.recovery.class.staging", "Unfinished upload"),
  logs: (t) => t("storagePanel.recovery.class.logs", "Logs"),
  derived: (t) => t("storagePanel.recovery.class.derived", "Derived files"),
  unknown: (t) => t("storagePanel.recovery.class.unknown", "Unknown file"),
};

const DISPOSITION_LABELS: Record<RecoveryDisposition, (t: TFunction) => string> = {
  preserved: (t) => t("storagePanel.recovery.disposition.preserved", "Kept in place"),
  moved: (t) => t("storagePanel.recovery.disposition.moved", "Kept for export"),
  reassociated: (t) => t("storagePanel.recovery.disposition.reassociated", "Attached to its photo"),
  ambiguous: (t) => t("storagePanel.recovery.disposition.ambiguous", "Needs review"),
  unreadable: (t) => t("storagePanel.recovery.disposition.unreadable", "Newer format"),
  deleted: (t) => t("storagePanel.recovery.disposition.deleted", "Deleted"),
};

export function isRecoveryItemClass(value: string | undefined): value is RecoveryItemClass {
  return value != null && Object.hasOwn(CLASS_LABELS, value);
}

export function isRecoveryDisposition(value: string | undefined): value is RecoveryDisposition {
  return value != null && Object.hasOwn(DISPOSITION_LABELS, value);
}

export function recoveryClassLabel(t: TFunction, value: string | undefined): string {
  return isRecoveryItemClass(value) ? CLASS_LABELS[value](t) : CLASS_LABELS.unknown(t);
}

export function recoveryDispositionLabel(t: TFunction, value: string | undefined): string {
  return isRecoveryDisposition(value)
    ? DISPOSITION_LABELS[value](t)
    : DISPOSITION_LABELS.preserved(t);
}

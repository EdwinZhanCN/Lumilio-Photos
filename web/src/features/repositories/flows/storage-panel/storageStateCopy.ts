import type { TFunction } from "i18next";
import type { RepositoryState, VerificationState } from "./storageViewModel";

/**
 * Label per Repository state. An explicit record, not string interpolation:
 * snake_case states (`identity_error`) and camelCase locale keys
 * (`identityError`) do not agree, and a missing entry must fail the type check
 * rather than let a badge fall back to the raw enum. Each entry calls `t` with
 * a literal key so `i18next-cli extract` keeps the key.
 */
const STATE_LABELS: Record<RepositoryState, (t: TFunction) => string> = {
  available: (t) => t("storagePanel.state.available", "Available"),
  scanning: (t) => t("storagePanel.state.scanning", "Scanning"),
  importing: (t) => t("storagePanel.state.importing", "Importing"),
  low_space: (t) => t("storagePanel.state.lowSpace", "Low space"),
  paused: (t) => t("storagePanel.state.paused", "Paused"),
  read_only: (t) => t("storagePanel.state.readOnly", "Read-only"),
  offline: (t) => t("storagePanel.state.offline", "Offline"),
  identity_error: (t) => t("storagePanel.state.identityError", "Identity mismatch"),
  recovery_required: (t) => t("storagePanel.state.recoveryRequired", "Recovery required"),
  maintenance: (t) => t("storagePanel.state.maintenance", "Maintenance"),
};

const VERIFICATION_LABELS: Record<VerificationState, (t: TFunction) => string> = {
  never: (t) => t("storagePanel.verificationBadge.never", "Not scanned"),
  verified: (t) => t("storagePanel.verificationBadge.verified", "Scanned"),
  partial: (t) => t("storagePanel.verificationBadge.partial", "Partial"),
  failed: (t) => t("storagePanel.verificationBadge.failed", "Failed"),
};

/** Localized label for one Repository state. Copy lives in the locale files. */
export function storageStateLabel(t: TFunction, state: RepositoryState): string {
  return STATE_LABELS[state](t);
}

/**
 * daisyUI badge color class per state. Only states needing action get a color;
 * a healthy row stays neutral so color never becomes decoration.
 */
export function storageStateBadgeClass(state: RepositoryState): string {
  switch (state) {
    case "identity_error":
    case "recovery_required":
      return "badge-error";
    case "offline":
    case "read_only":
    case "low_space":
    case "paused":
      return "badge-warning";
    case "scanning":
    case "importing":
    case "maintenance":
      return "badge-info";
    case "available":
    default:
      return "badge-ghost";
  }
}

export function verificationLabel(t: TFunction, state: VerificationState): string {
  return VERIFICATION_LABELS[state](t);
}

export function verificationBadgeClass(state: VerificationState): string {
  switch (state) {
    case "failed":
      return "badge-error";
    case "partial":
      return "badge-warning";
    case "verified":
      return "badge-ghost";
    case "never":
    default:
      return "badge-ghost";
  }
}

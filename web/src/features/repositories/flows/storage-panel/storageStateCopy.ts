import type { RepositoryState, VerificationState } from "./storageViewModel";

/**
 * i18n key per Repository state. An explicit record, not string interpolation:
 * snake_case states (`identity_error`) and camelCase locale keys
 * (`identityError`) do not agree, and a missing entry must fail the type check
 * rather than let a badge fall back to the raw enum.
 */
const STATE_LABEL_KEYS: Record<RepositoryState, string> = {
  available: "storagePanel.state.available",
  scanning: "storagePanel.state.scanning",
  importing: "storagePanel.state.importing",
  low_space: "storagePanel.state.lowSpace",
  paused: "storagePanel.state.paused",
  read_only: "storagePanel.state.readOnly",
  offline: "storagePanel.state.offline",
  identity_error: "storagePanel.state.identityError",
  recovery_required: "storagePanel.state.recoveryRequired",
  maintenance: "storagePanel.state.maintenance",
};

const VERIFICATION_LABEL_KEYS: Record<VerificationState, string> = {
  never: "storagePanel.verificationBadge.never",
  verified: "storagePanel.verificationBadge.verified",
  partial: "storagePanel.verificationBadge.partial",
  failed: "storagePanel.verificationBadge.failed",
};

/** i18n key for one Repository state. Copy lives in the locale files, not here. */
export function storageStateLabelKey(state: RepositoryState): string {
  return STATE_LABEL_KEYS[state];
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

export function verificationLabelKey(state: VerificationState): string {
  return VERIFICATION_LABEL_KEYS[state];
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

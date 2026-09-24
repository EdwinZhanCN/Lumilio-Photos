import type { TFunction } from "i18next";
import type { components } from "@/lib/http-commons/schema";
import type {
  AdmissionDecision,
  RepositoryAdmissionReason,
  RepositoryEffectiveState,
  RepositoryOption,
} from "../types";
import { normalizeRepositoryRole } from "./storageEntities";

type StorageTargetsResponse = components["schemas"]["dto.StorageTargetsResponseDTO"];

const ADMISSION_STATE_PRIORITY: readonly RepositoryAdmissionReason[] = [
  "identity_error",
  "recovery_required",
  "offline",
  "busy",
  "paused",
  "read_only",
  "low_space",
];

export function normalizeAdmissionDecision(
  decision?: components["schemas"]["dto.AdmissionDecisionDTO"],
): AdmissionDecision {
  return {
    allowed: decision?.allowed === true,
    reasons: decision?.reasons ?? [],
  };
}

export function normalizeRepositoryOptions(data?: StorageTargetsResponse): RepositoryOption[] {
  return (data?.targets ?? []).map((target) => ({
    entityType: "repository",
    id: target.id ?? "",
    rawName: target.name ?? "",
    role: normalizeRepositoryRole(target.role),
    read: normalizeAdmissionDecision(target.read),
    upload: normalizeAdmissionDecision(target.upload),
  }));
}

export function isRepositoryUnavailable(repository: RepositoryOption): boolean {
  return !repository.upload.allowed;
}

export function isUploadLowSpaceBlocked(repository: RepositoryOption): boolean {
  return (
    !repository.upload.allowed &&
    repository.upload.reasons.some((reason) => reason === "low_space" || reason === "paused")
  );
}

export function getRepositoryEffectiveState(
  repository: RepositoryOption,
): RepositoryEffectiveState {
  if (repository.upload.allowed) {
    return "active";
  }

  const reasons = new Set([
    ...repository.upload.reasons,
    ...(repository.read.allowed ? [] : repository.read.reasons),
  ]);

  for (const reason of ADMISSION_STATE_PRIORITY) {
    if (reasons.has(reason)) {
      return reason;
    }
  }

  return "blocked";
}

/**
 * Label per admission reason. Each entry calls `t` with a literal key so
 * `i18next-cli extract` keeps the key.
 */
const ADMISSION_REASON_LABELS: Record<RepositoryAdmissionReason, (t: TFunction) => string> = {
  offline: (t) => t("manage.repositories.offlineBadge", "Offline"),
  identity_error: (t) =>
    t("manage.repositories.identityErrorBadge", "Repository identity mismatch"),
  recovery_required: (t) => t("manage.repositories.recoveryRequiredBadge", "Recovery required"),
  busy: (t) => t("manage.repositories.busyBadge", "Busy"),
  paused: (t) => t("manage.repositories.pausedBadge", "Paused"),
  read_only: (t) => t("manage.repositories.readOnlyBadge", "Read only"),
  low_space: (t) => t("manage.repositories.lowSpaceBadge", "Low space"),
};

/**
 * Badge colour for an upload target's state. Unlike the storage table, where a
 * healthy row stays neutral, the upload target states its eligibility: green
 * means the batch can go here.
 */
export function uploadStateBadgeClass(state: RepositoryEffectiveState): string {
  switch (state) {
    case "active":
      return "badge-success";
    case "identity_error":
    case "recovery_required":
    case "blocked":
      return "badge-error";
    case "offline":
    case "read_only":
    case "low_space":
    case "paused":
    case "busy":
      return "badge-warning";
    default:
      return "badge-ghost";
  }
}

/**
 * Localized label for an admission reason. A reason the client does not know
 * yet falls back to the raw server value.
 */
export function uploadAdmissionReasonLabel(t: TFunction, reason: string): string {
  const label = ADMISSION_REASON_LABELS[reason as RepositoryAdmissionReason];
  return label ? label(t) : reason;
}

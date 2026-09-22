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

const ADMISSION_REASON_COPY: Record<
  RepositoryAdmissionReason,
  { key: string; defaultValue: string }
> = {
  offline: { key: "manage.repositories.offlineBadge", defaultValue: "Offline" },
  identity_error: {
    key: "manage.repositories.identityErrorBadge",
    defaultValue: "Repository identity mismatch",
  },
  recovery_required: {
    key: "manage.repositories.recoveryRequiredBadge",
    defaultValue: "Recovery required",
  },
  busy: { key: "manage.repositories.busyBadge", defaultValue: "Busy" },
  paused: { key: "manage.repositories.pausedBadge", defaultValue: "Paused" },
  read_only: { key: "manage.repositories.readOnlyBadge", defaultValue: "Read only" },
  low_space: { key: "manage.repositories.lowSpaceBadge", defaultValue: "Low space" },
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

export function uploadAdmissionReasonCopy(reason: string): { key: string; defaultValue: string } {
  return (
    ADMISSION_REASON_COPY[reason as RepositoryAdmissionReason] ?? {
      key: `manage.repositories.admissionReason.${reason}`,
      defaultValue: reason,
    }
  );
}

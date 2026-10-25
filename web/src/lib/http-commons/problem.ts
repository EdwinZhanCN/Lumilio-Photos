import type { TFunction } from "i18next";
import type { components } from "./schema";

export type KnownProblem = components["schemas"]["api.ProblemResponse"];
export type KnownProblemType = KnownProblem["type"];
export type KnownProblemReference = components["schemas"]["api.ProblemReference"];

const INSTANCE_PATTERN = /^urn:lumilio:problem:[0-9a-f]{32}$/;

const KNOWN_TYPES = {
  "about:blank": true,
  "https://lumilio.org/problems/asset/missing": true,
  "https://lumilio.org/problems/asset/offline": true,
  "https://lumilio.org/problems/asset/trashed": true,
  "https://lumilio.org/problems/agent/operation-failed": true,
  "https://lumilio.org/problems/auth/authentication-required": true,
  "https://lumilio.org/problems/auth/invalid-credentials": true,
  "https://lumilio.org/problems/auth/mfa-invalid": true,
  "https://lumilio.org/problems/auth/passkey-unavailable": true,
  "https://lumilio.org/problems/auth/permission-denied": true,
  "https://lumilio.org/problems/auth/rate-limited": true,
  "https://lumilio.org/problems/auth/session-expired": true,
  "https://lumilio.org/problems/auth/untrusted-origin": true,
  "https://lumilio.org/problems/backup/restore-failed": true,
  "https://lumilio.org/problems/bootstrap/app-not-initialized": true,
  "https://lumilio.org/problems/cloud/import-failed": true,
  "https://lumilio.org/problems/lumen/image-semantic-analysis-unavailable": true,
  "https://lumilio.org/problems/media/image-embedding-missing": true,
  "https://lumilio.org/problems/media/invalid-request": true,
  "https://lumilio.org/problems/repository/conflict": true,
  "https://lumilio.org/problems/repository/scan-failed": true,
  "https://lumilio.org/problems/repository/scan-incomplete": true,
  "https://lumilio.org/problems/repository/unavailable": true,
  "https://lumilio.org/problems/service/unavailable": true,
  "https://lumilio.org/problems/storage/action-unavailable": true,
  "https://lumilio.org/problems/storage/assessment-stale": true,
  "https://lumilio.org/problems/storage/busy": true,
  "https://lumilio.org/problems/storage/confirmation-required": true,
  "https://lumilio.org/problems/storage/host-action-expired": true,
  "https://lumilio.org/problems/storage/host-action-failed": true,
  "https://lumilio.org/problems/storage/identity-mismatch": true,
  "https://lumilio.org/problems/storage/layout-obstructed": true,
  "https://lumilio.org/problems/storage/materialization-required": true,
  "https://lumilio.org/problems/storage/name-collision": true,
  "https://lumilio.org/problems/storage/not-a-mounted-volume": true,
  "https://lumilio.org/problems/storage/observation-unknown": true,
  "https://lumilio.org/problems/storage/original-online": true,
  "https://lumilio.org/problems/storage/recovery-required": true,
  "https://lumilio.org/problems/storage/ownership-unavailable": true,
  "https://lumilio.org/problems/storage/permission-denied": true,
  "https://lumilio.org/problems/storage/unsupported-filesystem": true,
  "https://lumilio.org/problems/storage/volume-unqualified": true,
  "https://lumilio.org/problems/upload/processing-failed": true,
} as const satisfies Record<KnownProblemType, true>;

export type NormalizedProblem =
  | {
      kind: "problem";
      type: string;
      status: number;
      instance: string;
      retryAfterSeconds?: number;
      conflictType?: string;
      repositoryID?: string;
      actions?: readonly string[];
      risks?: readonly string[];
      assessmentToken?: string;
      action?: string;
      blockers?: readonly string[];
      alternatives?: readonly string[];
    }
  | { kind: "abort" }
  | { kind: "network" }
  | { kind: "malformed" };

export type NormalizedProblemReference = {
  type: string;
  instance: string;
  retryable?: boolean;
  retryAfterSeconds?: number;
  conflictType?: string;
  repositoryID?: string;
  actions?: readonly string[];
  risks?: readonly string[];
  assessmentToken?: string;
  action?: string;
  blockers?: readonly string[];
  alternatives?: readonly string[];
};

function record(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  return value as Record<string, unknown>;
}

function safeExtensions(source: Record<string, unknown>) {
  const retryAfterSeconds =
    Number.isInteger(source.retry_after_seconds) && Number(source.retry_after_seconds) > 0
      ? Number(source.retry_after_seconds)
      : undefined;
  const conflictType =
    typeof source.conflict_type === "string" && source.conflict_type.length <= 128
      ? source.conflict_type
      : undefined;
  const repositoryID =
    typeof source.repository_id === "string" && source.repository_id.length <= 128
      ? source.repository_id
      : undefined;
  const actions = boundedCodes(source.actions);
  const risks = boundedCodes(source.risks);
  const assessmentToken =
    typeof source.assessment_token === "string" &&
    source.assessment_token.length > 0 &&
    source.assessment_token.length <= 256
      ? source.assessment_token
      : undefined;
  const action =
    typeof source.action === "string" && source.action.length <= 64 ? source.action : undefined;
  const blockers = boundedCodes(source.blockers);
  const alternatives = boundedCodes(source.alternatives);
  return {
    retryAfterSeconds,
    conflictType,
    repositoryID,
    actions,
    risks,
    assessmentToken,
    action,
    blockers,
    alternatives,
  };
}

/** A short list of stable codes; anything longer or non-string is dropped whole. */
function boundedCodes(value: unknown): string[] | undefined {
  return Array.isArray(value) &&
    value.length <= 16 &&
    value.every((code) => typeof code === "string" && code.length <= 64)
    ? (value as string[]).slice()
    : undefined;
}

export function normalizeProblem(value: unknown): NormalizedProblem {
  if (isNormalizedProblem(value)) return value;
  if (value instanceof DOMException && value.name === "AbortError") return { kind: "abort" };
  if (value instanceof TypeError) return { kind: "network" };

  const source = record(value);
  if (!source) return { kind: "malformed" };
  if (
    typeof source.type !== "string" ||
    !Number.isInteger(source.status) ||
    Number(source.status) < 400 ||
    Number(source.status) > 599 ||
    typeof source.instance !== "string" ||
    !INSTANCE_PATTERN.test(source.instance)
  ) {
    return { kind: "malformed" };
  }

  return {
    kind: "problem",
    type: source.type,
    status: Number(source.status),
    instance: source.instance,
    ...safeExtensions(source),
  };
}

export function normalizeProblemReference(value: unknown): NormalizedProblemReference | undefined {
  const source = record(value);
  if (
    !source ||
    typeof source.type !== "string" ||
    typeof source.instance !== "string" ||
    !INSTANCE_PATTERN.test(source.instance)
  ) {
    return undefined;
  }
  const retryable = typeof source.retryable === "boolean" ? source.retryable : undefined;
  return { type: source.type, instance: source.instance, retryable, ...safeExtensions(source) };
}

export function getProblemType(value: unknown): string | undefined {
  const normalized = normalizeProblem(value);
  return normalized.kind === "problem" ? normalized.type : undefined;
}

export function getProblemInstance(value: unknown): string | undefined {
  const normalized = normalizeProblem(value);
  return normalized.kind === "problem" ? normalized.instance : undefined;
}

export function isAbortProblem(value: unknown): boolean {
  return normalizeProblem(value).kind === "abort";
}

export async function readProblemResponse(response: Response): Promise<NormalizedProblem> {
  if (response.ok) return { kind: "malformed" };
  const contentType = response.headers.get("Content-Type")?.split(";", 1)[0]?.trim();
  if (contentType !== "application/problem+json") return { kind: "malformed" };
  const payload: unknown = await response.json().catch(() => undefined);
  return normalizeProblem(payload);
}

export function localizeProblem(value: unknown, t: TFunction, operationFallback: string): string {
  const normalized = normalizeProblem(value);
  if (normalized.kind !== "problem" || !isKnownType(normalized.type)) return operationFallback;

  return localizeKnownProblem(normalized.type, normalized, t, operationFallback);
}

export function localizeProblemReference(
  value: unknown,
  t: TFunction,
  operationFallback: string,
): string {
  const normalized = normalizeProblemReference(value);
  if (!normalized || !isKnownType(normalized.type)) return operationFallback;
  return localizeKnownProblem(normalized.type, normalized, t, operationFallback);
}

export function localizeAPIProblem(
  value: unknown,
  t: TFunction,
  operationFallback: string,
): string {
  const normalized = normalizeProblem(value);
  if (normalized.kind === "problem") return localizeProblem(normalized, t, operationFallback);
  return localizeProblemReference(value, t, operationFallback);
}

/**
 * Localize a Problem type the Server reported outside a response, such as the
 * `problem` code of a storage assessment. Unknown types use the fallback.
 */
export function localizeProblemType(
  problemType: string,
  t: TFunction,
  operationFallback: string,
): string {
  if (!isKnownType(problemType)) return operationFallback;
  return localizeKnownProblem(problemType, {}, t, operationFallback);
}

function localizeKnownProblem(
  problemType: KnownProblemType,
  facts: {
    retryAfterSeconds?: number;
    conflictType?: string;
    blockers?: readonly string[];
  },
  t: TFunction,
  operationFallback: string,
): string {
  switch (problemType) {
    case "about:blank":
      return operationFallback;
    case "https://lumilio.org/problems/auth/invalid-credentials":
      return t("apiErrors.auth.invalidCredentials", "The username or password is incorrect.");
    case "https://lumilio.org/problems/auth/authentication-required":
      return t("apiErrors.auth.authenticationRequired", "Sign in to continue.");
    case "https://lumilio.org/problems/auth/session-expired":
      return t("apiErrors.auth.sessionExpired", "Your session expired. Sign in again.");
    case "https://lumilio.org/problems/auth/passkey-unavailable":
      return t(
        "apiErrors.auth.passkeyUnavailable",
        "Passkeys are unavailable in this browser or deployment.",
      );
    case "https://lumilio.org/problems/auth/mfa-invalid":
      return t("apiErrors.auth.mfaInvalid", "The verification code is invalid or expired.");
    case "https://lumilio.org/problems/auth/permission-denied":
      return t("apiErrors.auth.permissionDenied", "You do not have permission to do that.");
    case "https://lumilio.org/problems/auth/untrusted-origin":
      return t("apiErrors.auth.untrustedOrigin", "This browser origin is not trusted.");
    case "https://lumilio.org/problems/auth/rate-limited":
      return facts.retryAfterSeconds
        ? t(
            "apiErrors.auth.rateLimitedWithDelay",
            "Too many attempts. Try again in {{count}} seconds.",
            { count: facts.retryAfterSeconds },
          )
        : t("apiErrors.auth.rateLimited", "Too many attempts. Try again later.");
    case "https://lumilio.org/problems/bootstrap/app-not-initialized":
      return t("apiErrors.bootstrap.appNotInitialized", "Finish initial setup before continuing.");
    case "https://lumilio.org/problems/backup/restore-failed":
      return t(
        "apiErrors.backup.restoreFailed",
        "The database restore could not be completed. The previous database remains active.",
      );
    case "https://lumilio.org/problems/cloud/import-failed":
      return t("apiErrors.cloud.importFailed", "The cloud import could not be completed.");
    case "https://lumilio.org/problems/repository/unavailable":
      return t(
        "apiErrors.repository.unavailable",
        "The Repository required for this operation is unavailable.",
      );
    case "https://lumilio.org/problems/repository/conflict":
      return localizeRepositoryConflict(facts.conflictType, t);
    case "https://lumilio.org/problems/repository/scan-failed":
      return t("apiErrors.repository.scanFailed", "The Repository scan could not be completed.");
    case "https://lumilio.org/problems/repository/scan-incomplete":
      return t(
        "apiErrors.repository.scanIncomplete",
        "The Repository scan completed with partial results.",
      );
    case "https://lumilio.org/problems/storage/confirmation-required":
      return t(
        "apiErrors.storage.confirmationRequired",
        "Review and acknowledge the storage risks before continuing.",
      );
    case "https://lumilio.org/problems/storage/assessment-stale":
      return t(
        "apiErrors.storage.assessmentStale",
        "The storage target changed since it was reviewed. Review it again before continuing.",
      );
    case "https://lumilio.org/problems/storage/action-unavailable":
      return localizeStorageActionUnavailable(facts.blockers, t);
    case "https://lumilio.org/problems/storage/busy":
      return t(
        "apiErrors.storage.busy",
        "Another storage command is still working on this Repository. Wait for it to finish, then try again.",
      );
    case "https://lumilio.org/problems/storage/ownership-unavailable":
      return t(
        "apiErrors.storage.ownershipUnavailable",
        "Another Lumilio process owns this Repository folder. Stop it, then try again.",
      );
    case "https://lumilio.org/problems/storage/identity-mismatch":
      return t(
        "apiErrors.storage.identityMismatch",
        "The folder holds a different Repository than the one you chose. Nothing was changed.",
      );
    case "https://lumilio.org/problems/storage/original-online":
      return t(
        "apiErrors.storage.originalOnline",
        "This Repository is still available at its registered location, so there is nothing to reconnect.",
      );
    case "https://lumilio.org/problems/storage/volume-unqualified":
      return t(
        "apiErrors.storage.volumeUnqualified",
        "Lumilio could not determine how this disk compares file names, or folders inside the tree follow different rules, so the Repository was not placed there. Nothing was changed.",
      );
    case "https://lumilio.org/problems/storage/name-collision":
      return t(
        "apiErrors.storage.nameCollision",
        "This disk cannot tell some of the Repository's file names apart (they differ only by case or accent form), so the reconnect was refused before anything changed. Rename one of each pair on the original disk, or choose a disk that distinguishes them.",
      );
    case "https://lumilio.org/problems/storage/unsupported-filesystem":
      return t(
        "apiErrors.storage.unsupportedFilesystem",
        "Network storage (NFS, SMB, AFP, SSHFS, WebDAV, or a network drive) is not supported for Storage Locations and Repositories in this release. Choose a local disk.",
      );
    case "https://lumilio.org/problems/storage/not-a-mounted-volume":
      return t(
        "apiErrors.storage.notMountedVolume",
        "This folder is not a mounted volume; data stored here would be lost when the container is recreated. Mount a host directory in your compose file.",
      );
    case "https://lumilio.org/problems/storage/observation-unknown":
      return t(
        "apiErrors.storage.observationUnknown",
        "The storage target could not be inspected. Check the mount and access, then try again.",
      );
    case "https://lumilio.org/problems/storage/permission-denied":
      return t(
        "apiErrors.storage.permissionDenied",
        "Lumilio was denied access to the storage target. Grant access or choose another folder.",
      );
    case "https://lumilio.org/problems/storage/materialization-required":
      return t(
        "apiErrors.storage.materializationRequired",
        "Files in this folder must be downloaded locally before it can be used. Make the folder available offline and try again.",
      );
    case "https://lumilio.org/problems/storage/layout-obstructed":
      return t(
        "apiErrors.storage.layoutObstructed",
        "A file or a denied folder sits where a Repository folder belongs, so the layout was not repaired. Move it out of the way and review again; nothing was changed.",
      );
    case "https://lumilio.org/problems/storage/recovery-required":
      return t(
        "apiErrors.storage.recoveryRequired",
        "An interrupted storage operation on this Repository must be recovered first. Check its recovery inventory on the Storage page, then try again.",
      );
    case "https://lumilio.org/problems/storage/host-action-expired":
      return t(
        "apiErrors.storage.hostActionExpired",
        "The Desktop approval expired. Start the action again.",
      );
    case "https://lumilio.org/problems/storage/host-action-failed":
      return t(
        "apiErrors.storage.hostActionFailed",
        "The Desktop could not complete the storage action.",
      );
    case "https://lumilio.org/problems/service/unavailable":
      return t("apiErrors.service.unavailable", "A required service is unavailable.");
    case "https://lumilio.org/problems/asset/missing":
      return t(
        "apiErrors.asset.missing",
        "The original file is missing. Return the file and scan its Repository to restore it.",
      );
    case "https://lumilio.org/problems/asset/offline":
      return t(
        "apiErrors.asset.offline",
        "The Repository is unavailable. Reconnect it to open this asset.",
      );
    case "https://lumilio.org/problems/asset/trashed":
      return t(
        "apiErrors.asset.trashed",
        "This asset is in the Trash. Restore it to open the original.",
      );
    case "https://lumilio.org/problems/media/invalid-request":
      return t("apiErrors.media.invalidRequest", "The media request could not be processed.");
    case "https://lumilio.org/problems/media/image-embedding-missing":
      return t(
        "apiErrors.media.imageEmbeddingMissing",
        "The selected asset has not completed Image Semantic Analysis.",
      );
    case "https://lumilio.org/problems/lumen/image-semantic-analysis-unavailable":
      return t(
        "apiErrors.lumen.imageSemanticAnalysisUnavailable",
        "Image Semantic Analysis is unavailable.",
      );
    case "https://lumilio.org/problems/agent/operation-failed":
      return t("apiErrors.agent.operationFailed", "The Agent operation could not be completed.");
    case "https://lumilio.org/problems/upload/processing-failed":
      return t("apiErrors.upload.processingFailed", "The upload could not be processed.");
    default:
      return assertNever(problemType);
  }
}

/**
 * The first blocker names the reason an action is withheld; the Server lists
 * them most specific first. Unknown codes fall back to the generic refusal
 * rather than rendering a raw identifier. Exported so menus can explain a
 * withheld entry with the same copy the refused command would show.
 */
export function localizeStorageActionBlockers(
  blockers: readonly string[] | undefined,
  t: TFunction,
): string {
  return localizeStorageActionUnavailable(blockers, t);
}

/**
 * Localized sentence for the legal entries the Server lists instead of a
 * withheld action (`alternatives` of storage/action-unavailable and of the
 * storage view's withheld entries). Every alternative code the policy emits
 * has a literal case here; an empty list reads as nothing.
 */
export function localizeStorageActionAlternatives(
  alternatives: readonly string[] | undefined,
  t: TFunction,
): string {
  return (alternatives ?? [])
    .map((alternative) => localizeStorageActionAlternative(alternative, t))
    .filter(Boolean)
    .join(" ");
}

function localizeStorageActionAlternative(alternative: string, t: TFunction): string {
  switch (alternative) {
    case "default_configuration":
      return t(
        "apiErrors.storage.alternatives.defaultConfiguration",
        "Change the Default Storage Location through the storage configuration.",
      );
    case "recover_operation":
      return t(
        "apiErrors.storage.alternatives.recoverOperation",
        "Let the interrupted operation recover, then try again.",
      );
    case "wait_for_idle":
      return t(
        "apiErrors.storage.alternatives.waitForIdle",
        "Wait for the current work to finish, then try again.",
      );
    case "check_access":
      return t(
        "apiErrors.storage.alternatives.checkAccess",
        "Check access to the registered location, then try again.",
      );
    default:
      return "";
  }
}

function localizeStorageActionUnavailable(
  blockers: readonly string[] | undefined,
  t: TFunction,
): string {
  switch (blockers?.[0]) {
    case "primary_protected":
      return t(
        "apiErrors.storage.actionUnavailable.primaryProtected",
        "The Primary Repository follows the Default Storage Location. Change it through the storage configuration instead.",
      );
    case "repository_offline":
      return t(
        "apiErrors.storage.actionUnavailable.repositoryOffline",
        "The Repository is offline. Reconnect it or remove it from Lumilio instead.",
      );
    case "identity_problem":
      return t(
        "apiErrors.storage.actionUnavailable.identityProblem",
        "The Repository marker at the registered location needs attention before this action can run.",
      );
    case "recovery_required":
      return t(
        "apiErrors.storage.actionUnavailable.recoveryRequired",
        "An interrupted storage operation must be recovered before this action can run.",
      );
    case "maintenance":
      return t(
        "apiErrors.storage.actionUnavailable.maintenance",
        "The Repository is being removed. Wait for that to finish.",
      );
    case "repository_busy":
      return t(
        "apiErrors.storage.actionUnavailable.repositoryBusy",
        "The Repository is busy with a scan or import. Wait for it to finish, then try again.",
      );
    case "pending_operation":
      return t(
        "apiErrors.storage.actionUnavailable.pendingOperation",
        "Another storage operation on this Repository is still running. Wait for it to finish, then try again.",
      );
    case "original_available":
      return t(
        "apiErrors.storage.actionUnavailable.originalAvailable",
        "This Repository is still available at its registered location, so there is nothing to reconnect.",
      );
    case "original_inaccessible":
      return t(
        "apiErrors.storage.actionUnavailable.originalInaccessible",
        "The registered location could not be read, so Lumilio cannot tell whether it is gone. Check access to it, then try again.",
      );
    case "default_protected":
      return t(
        "apiErrors.storage.actionUnavailable.defaultProtected",
        "The Default Storage Location follows the configured storage path. Change it through the storage configuration instead.",
      );
    case "registered_repositories":
      return t(
        "apiErrors.storage.actionUnavailable.registeredRepositories",
        "This Storage Location still registers Repositories. Remove them from Lumilio first.",
      );
    default:
      return t(
        "apiErrors.storage.actionUnavailable.generic",
        "This action is not available for the Repository right now.",
      );
  }
}

function localizeRepositoryConflict(conflictType: string | undefined, t: TFunction): string {
  switch (conflictType) {
    case "file_changed":
      return t(
        "apiErrors.asset.fileChanged",
        "A file changed since the last scan. Scan its Repository and try again; no files were moved.",
      );
    case "not_trashed":
      return t("apiErrors.asset.notTrashed", "The selected asset has no file in the Trash.");
    case "move_failed":
      return t(
        "apiErrors.asset.moveFailed",
        "The files could not be moved. The catalog was left unchanged.",
      );
    case "repository_identity":
      return t(
        "apiErrors.repository.conflicts.identity",
        "This Repository identity is already registered.",
      );
    case "existing_repository_found":
      return t(
        "apiErrors.repository.conflicts.existingFound",
        "An existing Repository was found in this Storage Location.",
      );
    case "repository_marker_invalid":
      return t(
        "apiErrors.repository.conflicts.invalidMarker",
        "The selected Repository marker needs attention.",
      );
    case "storage_location_offline":
      return t(
        "apiErrors.repository.conflicts.storageLocationOffline",
        "The Storage Location is offline.",
      );
    default:
      return t(
        "apiErrors.repository.conflicts.generic",
        "The Repository needs attention before continuing.",
      );
  }
}

function isKnownType(value: string): value is KnownProblemType {
  return Object.hasOwn(KNOWN_TYPES, value);
}

function isNormalizedProblem(value: unknown): value is NormalizedProblem {
  const source = record(value);
  return Boolean(
    source &&
    (source.kind === "problem" ||
      source.kind === "abort" ||
      source.kind === "network" ||
      source.kind === "malformed"),
  );
}

function assertNever(value: never): never {
  throw new Error(`Unhandled generated Problem type: ${String(value)}`);
}

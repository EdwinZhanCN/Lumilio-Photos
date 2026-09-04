import type { TFunction } from "i18next";
import type { components } from "./schema";

export type KnownProblem = components["schemas"]["api.ProblemResponse"];
export type KnownProblemType = KnownProblem["type"];
export type KnownProblemReference = components["schemas"]["api.ProblemReference"];

const INSTANCE_PATTERN = /^urn:lumilio:problem:[0-9a-f]{32}$/;

const KNOWN_TYPES = {
  "about:blank": true,
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
  "https://lumilio.org/problems/storage/confirmation-required": true,
  "https://lumilio.org/problems/storage/host-action-expired": true,
  "https://lumilio.org/problems/storage/host-action-failed": true,
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
  const actions =
    Array.isArray(source.actions) &&
    source.actions.length <= 16 &&
    source.actions.every((action) => typeof action === "string" && action.length <= 64)
      ? (source.actions as string[]).slice()
      : undefined;
  return { retryAfterSeconds, conflictType, repositoryID, actions };
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

function localizeKnownProblem(
  problemType: KnownProblemType,
  facts: {
    retryAfterSeconds?: number;
    conflictType?: string;
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
        "Review and confirm the Storage Location risks before continuing.",
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

function localizeRepositoryConflict(conflictType: string | undefined, t: TFunction): string {
  switch (conflictType) {
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

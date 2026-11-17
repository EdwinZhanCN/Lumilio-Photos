import type { components } from "@/lib/http-commons/schema";

type StorageLocationViewDTO = components["schemas"]["dto.StorageLocationViewDTO"];
type StorageDiagnosticDTO = components["schemas"]["dto.StorageDiagnosticDTO"];

export type StorageLocationKind = "default" | "external" | "unknown";
export type RepositoryRole = "primary" | "regular" | "unknown";

export type AdmissionDecision = {
  allowed: boolean;
  reasons: readonly string[];
};

export type StorageLocationEntity = {
  entityType: "storage_location";
  kind: StorageLocationKind;
  rawName: string;
  path: string;
};

export type RepositoryEntity = {
  entityType: "repository";
  role: RepositoryRole;
  rawName: string;
  path: string;
};

export type UnknownStorageEntity = {
  entityType: "unknown";
  rawName: string;
  path: string;
};

export type StorageEntity = StorageLocationEntity | RepositoryEntity | UnknownStorageEntity;

export type StorageLocationOption = Omit<StorageLocationViewDTO, "kind" | "name"> &
  StorageLocationEntity & {
    id: string;
  };

export type StorageLocationsResponse = {
  storage_locations?: StorageLocationOption[];
};

export type StorageDiagnostic = Omit<StorageDiagnosticDTO, "kind" | "name" | "path" | "role"> &
  StorageEntity;

export type StorageDiagnosticsResponse = {
  generated_at?: string;
  items?: StorageDiagnostic[];
};

export type RepositoryOption = {
  entityType: "repository";
  id: string;
  rawName: string;
  role: RepositoryRole;
  read: AdmissionDecision;
  upload: AdmissionDecision;
};

/**
 * The identity a command needs to target one Repository. Commands that only
 * name an existing Repository take this instead of {@link RepositoryOption}:
 * admission is a Server fact served by `/storage/targets`, so a command must
 * not carry locally invented eligibility.
 */
export type RepositoryRef = {
  id: string;
  rawName: string;
  role: RepositoryRole;
};

/** Closed Server admission reasons plus local presentation fallbacks. */
export type RepositoryAdmissionReason =
  | "offline"
  | "identity_error"
  | "read_only"
  | "low_space"
  | "paused"
  | "busy"
  | "recovery_required";

export type RepositoryEffectiveState = "active" | RepositoryAdmissionReason | "blocked";

/** @deprecated Use RepositoryEffectiveState. Kept as a public alias. */
export type RepositoryReachability = RepositoryEffectiveState;

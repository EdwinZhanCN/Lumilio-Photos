import type { components } from "@/lib/http-commons/schema";

type RepositoryRootDTO = components["schemas"]["dto.RepositoryRootDTO"];
type StorageDiagnosticDTO = components["schemas"]["dto.StorageDiagnosticDTO"];

export type StorageLocationKind = "default" | "external" | "unknown";
export type RepositoryRole = "primary" | "regular" | "unknown";

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

export type StorageLocationOption = Omit<RepositoryRootDTO, "kind" | "name" | "path"> &
  StorageLocationEntity;

export type RepositoryRootsResponse = {
  roots?: StorageLocationOption[];
};

export type StorageDiagnostic = Omit<StorageDiagnosticDTO, "kind" | "name" | "path" | "role"> &
  StorageEntity;

export type StorageDiagnosticsResponse = {
  generated_at?: string;
  items?: StorageDiagnostic[];
};

export type RepositoryOption = RepositoryEntity & {
  id: string;
  rootId: string;
  /**
   * Reachability of the repository's on-disk location. Offline and invalid
   * repositories stay selectable as browse filters but are not upload targets.
   */
  reachability: RepositoryReachability;
  /** Work is orthogonal to reachability and never hides an unavailable state. */
  activity: RepositoryActivity;
  pauseReason?: string;
};

export type RepositoryReachability =
  | "active"
  | "offline"
  | "identity_error"
  | "recovery_required"
  | "maintenance";

export type RepositoryActivity = "idle" | "scanning" | "importing" | "processing" | "paused";

export type RepositoryEffectiveState =
  | RepositoryReachability
  | "storage_location_offline"
  | "storage_location_error"
  | "storage_location_maintenance";

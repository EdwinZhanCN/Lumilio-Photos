import type { components } from "@/lib/http-commons/schema";
import type {
  RepositoryOption,
  RepositoryRole,
  StorageLocationsResponse,
  StorageDiagnostic,
  StorageDiagnosticsResponse,
  StorageEntity,
  StorageLocationKind,
} from "../types";

type StorageViewResponseDTO = components["schemas"]["dto.StorageViewResponseDTO"];
type StorageDiagnosticsDTO = components["schemas"]["dto.StorageDiagnosticsResponseDTO"];

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

export function getStorageEntityDisplayName(
  entity: StorageEntity | RepositoryOption,
  t: TranslateFn,
): string {
  if (entity.entityType === "storage_location" && entity.kind === "default") {
    return t("productTerms.defaultStorageLocation", {
      defaultValue: "Default Storage Location",
    });
  }

  if (entity.entityType === "repository" && entity.role === "primary") {
    return t("productTerms.primaryRepository", {
      defaultValue: "Primary Repository",
    });
  }

  // A Repository selector carries no path, so the fallback applies only when
  // the entity actually has one.
  const fallback = "path" in entity ? entity.path : "";
  return entity.rawName || fallback;
}

export function normalizeStorageLocationKind(kind?: string): StorageLocationKind {
  if (kind === "default" || kind === "external") return kind;
  return "unknown";
}

export function normalizeRepositoryRole(role?: string, isPrimary = false): RepositoryRole {
  if (isPrimary || role === "primary") return "primary";
  if (!role || role === "regular") return "regular";
  return "unknown";
}

export function normalizeStorageViewLocations(
  data: StorageViewResponseDTO,
): StorageLocationsResponse {
  return {
    storage_locations: (data.storage_locations ?? []).map(({ kind, name, id, ...location }) => ({
      ...location,
      id: id ?? "",
      entityType: "storage_location",
      kind: normalizeStorageLocationKind(kind),
      rawName: name ?? "",
      path: "",
    })),
  };
}

export function normalizeStorageDiagnosticsResponse(
  data: StorageDiagnosticsDTO,
): StorageDiagnosticsResponse {
  return {
    ...data,
    items: (data.items ?? []).map(normalizeStorageDiagnostic),
  };
}

function normalizeStorageDiagnostic({
  kind,
  name,
  path,
  role,
  ...diagnostic
}: components["schemas"]["dto.StorageDiagnosticDTO"]): StorageDiagnostic {
  const shared = {
    ...diagnostic,
    rawName: name ?? "",
    path: path ?? "",
  };

  if (diagnostic.target_type === "storage_location") {
    return {
      ...shared,
      entityType: "storage_location",
      kind: normalizeStorageLocationKind(kind),
    };
  }

  if (diagnostic.target_type === "repository") {
    return {
      ...shared,
      entityType: "repository",
      role: normalizeRepositoryRole(role),
    };
  }

  return {
    ...shared,
    entityType: "unknown",
  };
}

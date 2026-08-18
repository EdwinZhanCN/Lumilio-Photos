import type { components } from "@/lib/http-commons/schema";
import type {
  RepositoryRole,
  RepositoryRootsResponse,
  StorageDiagnostic,
  StorageDiagnosticsResponse,
  StorageEntity,
  StorageLocationKind,
} from "../types";

type RepositoryRootsDTO = components["schemas"]["dto.ListRepositoryRootsResponseDTO"];
type StorageDiagnosticsDTO = components["schemas"]["dto.StorageDiagnosticsResponseDTO"];

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

export function getStorageEntityDisplayName(entity: StorageEntity, t: TranslateFn): string {
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

  return entity.rawName || entity.path;
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

export function normalizeRepositoryRootsResponse(
  data: RepositoryRootsDTO,
): RepositoryRootsResponse {
  return {
    roots: (data.roots ?? []).map(({ kind, name, path, ...root }) => ({
      ...root,
      entityType: "storage_location",
      kind: normalizeStorageLocationKind(kind),
      rawName: name ?? "",
      path: path ?? "",
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

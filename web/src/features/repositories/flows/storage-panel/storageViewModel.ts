import type { components } from "@/lib/http-commons/schema";
import type { StorageViewResponse } from "../../api/useStorageView";
import { normalizeRepositoryRole } from "../../model/storageEntities";
import { deriveStorageLabel, type StorageLabel } from "../../model/storageLabels";

type StorageRepositoryViewDTO = components["schemas"]["dto.StorageRepositoryViewDTO"];
type StorageLocationViewDTO = components["schemas"]["dto.StorageLocationViewDTO"];
type StorageCapacityGroupDTO = components["schemas"]["dto.StorageCapacityGroupDTO"];

/**
 * One Repository's availability. Reading and writing are separate facts, but
 * the page shows one dominant state per row; the Inspector keeps the write
 * policy detail.
 */
export type RepositoryState =
  | "available"
  | "scanning"
  | "importing"
  | "low_space"
  | "paused"
  | "read_only"
  | "offline"
  | "identity_error"
  | "recovery_required"
  | "maintenance";

export type VerificationState = "never" | "verified" | "partial" | "failed";

export type RepositoryCapacity = {
  capacityGroupId: string;
  totalBytes: number | null;
  availableBytes: number | null;
  capacityKnown: boolean;
  groupingKnown: boolean;
};

/**
 * A Repository row. Capacity travels with the row because it is measured at
 * that Repository's path: one Storage Location can hold Repositories on
 * different mounts, so a Location-level figure would be wrong for at least one
 * of them. The storage label names where the figure was measured.
 */
export type RepositoryRow = {
  id: string;
  name: string;
  role: "primary" | "regular";
  state: RepositoryState;
  assetCount: number;
  verification: VerificationState;
  storageLocationId: string | null;
  storage: StorageLabel;
  filesystem: string;
  capacity: RepositoryCapacity | null;
};

export type StorageLocationGroup = {
  id: string;
  name: string;
  kind: "default" | "external" | "unknown";
  canRemove: boolean;
  blockingReason?: string;
  repositories: RepositoryRow[];
};

export type StorageView = {
  locations: StorageLocationGroup[];
  /** Repositories whose Storage Location is not registered in this instance. */
  unlinked: RepositoryRow[];
  observedAt: string | null;
  repositoryCount: number;
  /** Rows a reader must act on: anything that is not plainly available. */
  attentionCount: number;
};

export const EMPTY_STORAGE_VIEW: StorageView = {
  locations: [],
  unlinked: [],
  observedAt: null,
  repositoryCount: 0,
  attentionCount: 0,
};

function mapVerification(
  verification: StorageRepositoryViewDTO["verification"],
): VerificationState {
  switch (verification?.status) {
    case undefined:
    case "":
      return "never";
    case "failed":
      return "failed";
    case "partial":
      return "partial";
    default:
      return "verified";
  }
}

function mapRepositoryState(repository: StorageRepositoryViewDTO): RepositoryState {
  const reachability = repository.reachability ?? "active";
  if (reachability === "offline") return "offline";
  if (reachability === "identity_error") return "identity_error";
  if (reachability === "recovery_required") return "recovery_required";
  if (reachability === "maintenance") return "maintenance";
  if (reachability === "read_only") return "read_only";

  const activity = repository.activity ?? repository.write_policy?.activity ?? "idle";
  if (activity === "scanning") return "scanning";
  if (activity === "importing") return "importing";
  if (activity === "paused") return "paused";

  if (repository.write_policy?.pause_reason === "low_space") return "low_space";

  return "available";
}

type CapacityFacts = Omit<RepositoryCapacity, "capacityGroupId">;

function mapCapacityGroups(
  groups: StorageCapacityGroupDTO[] | undefined,
): Map<string, CapacityFacts> {
  const byId = new Map<string, CapacityFacts>();
  for (const group of groups ?? []) {
    const id = group.id;
    if (!id) continue;
    byId.set(id, {
      totalBytes: group.total_bytes ?? null,
      availableBytes: group.available_bytes ?? null,
      capacityKnown: group.capacity_known === true,
      groupingKnown: group.grouping_known === true,
    });
  }
  return byId;
}

function mapRepositoryRow(
  repository: StorageRepositoryViewDTO,
  capacityGroups: Map<string, CapacityFacts>,
): RepositoryRow | null {
  const id = repository.id;
  if (!id) return null;

  const role = normalizeRepositoryRole(repository.role);
  if (role !== "primary" && role !== "regular") return null;

  const capacityGroupId = repository.capacity_group_id;
  const group = capacityGroupId ? capacityGroups.get(capacityGroupId) : undefined;

  return {
    id,
    name: repository.name ?? id,
    role,
    state: mapRepositoryState(repository),
    assetCount: repository.asset_count ?? 0,
    verification: mapVerification(repository.verification),
    storageLocationId: repository.storage_location_id ?? null,
    storage: deriveStorageLabel(repository.mount_path),
    filesystem: repository.filesystem ?? "",
    capacity: capacityGroupId && group ? { capacityGroupId, ...group } : null,
  };
}

/**
 * Builds the page model: Storage Location groups, each holding the Repositories
 * that belong to it. Every Repository appears exactly once. Nothing is grouped
 * by backing storage — that identity travels per row as a label, so the page
 * structure never changes shape with the data.
 */
export function buildStorageView(data?: StorageViewResponse): StorageView {
  if (!data) return EMPTY_STORAGE_VIEW;

  const capacityGroups = mapCapacityGroups(data.capacity_groups);
  const rows = (data.repositories ?? [])
    .map((repository) => mapRepositoryRow(repository, capacityGroups))
    .filter((row): row is RepositoryRow => row != null);

  const locationsById = new Map<string, StorageLocationViewDTO>();
  for (const location of data.storage_locations ?? []) {
    if (location.id) locationsById.set(location.id, location);
  }

  const rowsByLocation = new Map<string, RepositoryRow[]>();
  const unlinked: RepositoryRow[] = [];
  for (const row of rows) {
    const locationId = row.storageLocationId;
    if (locationId && locationsById.has(locationId)) {
      const list = rowsByLocation.get(locationId) ?? [];
      list.push(row);
      rowsByLocation.set(locationId, list);
    } else {
      unlinked.push(row);
    }
  }

  const locations: StorageLocationGroup[] = [];
  for (const [id, location] of locationsById) {
    locations.push({
      id,
      name: location.name ?? id,
      kind: location.kind === "default" || location.kind === "external" ? location.kind : "unknown",
      canRemove: location.can_remove === true,
      blockingReason: location.blocking_reason,
      repositories: rowsByLocation.get(id) ?? [],
    });
  }

  return {
    locations,
    unlinked,
    observedAt: data.observed_at ?? null,
    repositoryCount: rows.length,
    attentionCount: rows.filter((row) => row.state !== "available").length,
  };
}

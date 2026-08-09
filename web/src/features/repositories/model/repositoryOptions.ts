import type { components } from "@/lib/http-commons/schema";
import type {
  RepositoryActivity,
  RepositoryEffectiveState,
  RepositoryOption,
  RepositoryReachability,
} from "../types";

type RepositoryListResponse = components["schemas"]["dto.IndexingRepositoryListResponseDTO"];

const REPOSITORY_REACHABILITY: RepositoryReachability[] = [
  "active",
  "offline",
  "identity_error",
  "recovery_required",
  "maintenance",
];
const REPOSITORY_ACTIVITIES: RepositoryActivity[] = [
  "idle",
  "scanning",
  "importing",
  "processing",
  "paused",
];

export function normalizeRepositoryOptions(data?: RepositoryListResponse): RepositoryOption[] {
  return (data?.repositories ?? []).map((repository) => ({
    id: repository.id ?? "",
    name: repository.name ?? "",
    path: repository.path ?? "",
    role: repository.role ?? "regular",
    rootId: repository.root_id ?? "",
    reachability: normalizeRepositoryReachability(repository.reachability),
    activity: normalizeRepositoryActivity(repository.activity),
    isPrimary: repository.role === "primary" || Boolean(repository.is_primary),
  }));
}

// Missing lifecycle state fails closed. A stale or partial response must never
// turn an unknown storage target into an eligible upload destination.
function normalizeRepositoryReachability(reachability?: string): RepositoryReachability {
  return (
    REPOSITORY_REACHABILITY.find((candidate) => candidate === reachability) ?? "recovery_required"
  );
}

function normalizeRepositoryActivity(activity?: string): RepositoryActivity {
  return REPOSITORY_ACTIVITIES.find((candidate) => candidate === activity) ?? "idle";
}

export function isRepositoryUnavailable(repository: RepositoryOption): boolean {
  return repository.reachability !== "active";
}

export function getRepositoryEffectiveState(
  repository: RepositoryOption,
  rootStatus?: string,
): RepositoryEffectiveState {
  if (rootStatus === "offline") return "storage_location_offline";
  if (rootStatus === "error") return "storage_location_error";
  if (rootStatus === "maintenance") return "storage_location_maintenance";
  return repository.reachability;
}

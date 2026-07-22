import type { components } from "@/lib/http-commons/schema";
import type { RepositoryOption, RepositoryStatus } from "../types";

type RepositoryListResponse = components["schemas"]["dto.IndexingRepositoryListResponseDTO"];

const REPOSITORY_STATUSES: RepositoryStatus[] = ["active", "scanning", "error", "offline"];

export function normalizeRepositoryOptions(data?: RepositoryListResponse): RepositoryOption[] {
  return (data?.repositories ?? []).map((repository) => ({
    id: repository.id ?? "",
    name: repository.name ?? "",
    path: repository.path ?? "",
    role: repository.role ?? "regular",
    status: normalizeRepositoryStatus(repository.status),
    isPrimary: repository.role === "primary" || Boolean(repository.is_primary),
  }));
}

// An unrecognized status must not read as unreachable: treating a repository as
// offline blocks uploads into it, so the safe default is "active".
function normalizeRepositoryStatus(status?: string): RepositoryStatus {
  return REPOSITORY_STATUSES.find((candidate) => candidate === status) ?? "active";
}

export function isRepositoryUnavailable(repository: RepositoryOption): boolean {
  return repository.status === "offline" || repository.status === "error";
}

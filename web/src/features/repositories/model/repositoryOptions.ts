import type { components } from "@/lib/http-commons/schema";
import type { RepositoryOption } from "../types";

type RepositoryListResponse = components["schemas"]["dto.IndexingRepositoryListResponseDTO"];

export function normalizeRepositoryOptions(data?: RepositoryListResponse): RepositoryOption[] {
  return (data?.repositories ?? []).map((repository) => ({
    id: repository.id ?? "",
    name: repository.name ?? "",
    path: repository.path ?? "",
    role: repository.role ?? "regular",
    isPrimary: repository.role === "primary" || Boolean(repository.is_primary),
  }));
}

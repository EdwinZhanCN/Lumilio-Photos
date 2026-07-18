/**
 * # Repositories
 *
 * Repositories owns the shared repository option contract, repository-aware
 * scope selection, scan lifecycle, and the repository management UI composed
 * by Manage. Consumers use the feature's root public entry; internal paths are
 * not public API.
 *
 * ## Data
 *
 * {@link useRepositoryOptions} reads the server's active repository list.
 * {@link normalizeRepositoryOptions} is the React-free DTO adapter that
 * produces {@link RepositoryOption} values for every consumer.
 * {@link useRepositoryScan} starts repository scans and stack detection, while
 * {@link waitForRepositoryScan} follows the requested scan run to a terminal
 * state before repository-aware queries are invalidated.
 *
 * ## Flows
 *
 * {@link BrowseScopeSelect} and {@link useBrowseScope} form the browse-scope
 * flow. An empty preference intentionally means all repositories and is valid
 * for list, map, collection, and statistics pages.
 *
 * {@link useWorkingRepository} forms the upload-target flow. It must resolve a
 * concrete repository and falls back to the primary or first active option.
 * Browse scope and working repository remain separate preferences because
 * their empty-state semantics differ.
 *
 * {@link RepositoryGrid} owns the repository-management surface. Its create
 * modal delegates server mutation and invalidation to
 * {@link useCreateRepository}; the grid receives maintenance commands from the
 * higher Manage composition route rather than importing those domains.
 *
 * ## State
 *
 * Repository lists are TanStack Query server state. Browse and working ids are
 * persisted user-scoped preferences through the lower preferences contract and
 * are cleared by authentication session reset. Scan/detection id sets are
 * request-local interaction state inside {@link useRepositoryScan}; fetched
 * repository data is never copied into Context or Zustand.
 *
 * @module
 */
import type { useCreateRepository } from "./api/useCreateRepository.ts";
import type { useRepositoryOptions } from "./api/useRepositoryOptions.ts";
import type { useRepositoryScan } from "./api/useRepositoryScan.ts";
import type { waitForRepositoryScan } from "./api/waitForRepositoryScan.ts";
import type BrowseScopeSelect from "./flows/browse-scope/BrowseScopeSelect.tsx";
import type { useBrowseScope } from "./flows/browse-scope/useBrowseScope.ts";
import type RepositoryGrid from "./flows/manage/RepositoryGrid.tsx";
import type { useWorkingRepository } from "./flows/working-repository/useWorkingRepository.ts";
import type { normalizeRepositoryOptions } from "./model/repositoryOptions.ts";
import type { RepositoryOption } from "./types.ts";

export {};

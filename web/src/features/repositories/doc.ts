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
 * {@link useRepositoryRoots} reads the admin-visible Storage Locations that
 * the native Desktop host has authorized; Web callers receive identities and
 * reachability, never an arbitrary host-path write capability.
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
 * concrete repository and falls back to the primary or first option that
 * {@link isRepositoryUnavailable} reports as writable — auto-selecting an
 * unreachable repository would guarantee the next upload is refused. An
 * explicit user choice is left alone even when it goes offline. Browse scope
 * and working repository remain separate preferences because their empty-state
 * semantics differ.
 *
 * ## Reachability
 *
 * {@link RepositoryStatus} carries a repository's reachability alongside its
 * activity. An `offline` repository is one whose folder is not currently
 * mounted — an unplugged external drive — while `error` means the on-disk
 * identity or config needs attention. Both stay selectable as browse filters
 * and are refused as write targets. An unrecognized status normalizes to
 * `active`, because a client-side guess must not incorrectly block a valid
 * repository.
 *
 * {@link RepositoryGrid} owns the repository-management surface. Its create
 * modal selects a registered root and sends explicit storage-layout and
 * duplicate-filename policies through {@link useCreateRepository}; local and
 * cloud creation share the same body, with only the cloud credential differing.
 * Auth setup uses the same public mutation for primary-repository creation.
 * {@link isStorageStrategy} keeps storage-policy parsing in the repository
 * model. The grid receives maintenance commands from the higher Manage
 * composition route rather than importing those domains.
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
import type { useRepositoryRoots } from "./api/useRepositoryRoots.ts";
import type { useRepositoryScan } from "./api/useRepositoryScan.ts";
import type { waitForRepositoryScan } from "./api/waitForRepositoryScan.ts";
import type BrowseScopeSelect from "./flows/browse-scope/BrowseScopeSelect.tsx";
import type { useBrowseScope } from "./flows/browse-scope/useBrowseScope.ts";
import type RepositoryGrid from "./flows/manage/RepositoryGrid.tsx";
import type { useWorkingRepository } from "./flows/working-repository/useWorkingRepository.ts";
import type {
  isRepositoryUnavailable,
  normalizeRepositoryOptions,
} from "./model/repositoryOptions.ts";
import type { isStorageStrategy } from "./model/repositorySetup.ts";
import type { RepositoryOption, RepositoryStatus } from "./types.ts";

export {};

/**
 * # Repositories
 *
 * Repositories owns the shared repository option contract, browse scope,
 * concrete upload destination, Storage Location selection, repository creation,
 * scan lifecycle, and repository-management UI composed by Manage.
 *
 * ## State
 *
 * Repository lists, roots, counts, and cloud status are TanStack Query server
 * state. Browse and working repository ids are user-scoped persisted
 * preferences and are cleared by authentication reset.
 *
 * {@link useBrowseScope} permits an empty value meaning all repositories.
 * {@link useWorkingRepository} must resolve one concrete reachable repository;
 * when no valid explicit choice exists it selects a reachable primary or first
 * option. An explicit choice remains selected if it later goes offline so the
 * application can explain the unavailable target rather than silently switch.
 * Scan/detection id sets are request-local interaction inside
 * {@link useRepositoryScan}.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     BROWSE["browse pages"] --> BSCOPE["BrowseScopeSelect"]
 *     UPLOAD["Upload"] --> WORKING["useWorkingRepository"]
 *     MANAGE["Manage"] --> GRID["RepositoryGrid"]
 *     GRID --> CREATE["AddRepositoryModal"]
 *     CREATE --> ROOTS["registered Storage Locations"]
 *     CREATE --> CLOUD["optional cloud credential"]
 *     MANAGE --> SCAN["scan / stack detection"]
 * ```
 *
 * {@link BrowseScopeSelect} is shared by read-oriented pages.
 * {@link RepositoryGrid} owns repository cards and creation UI but receives
 * maintenance commands from Manage. Creation selects an authorized root and
 * explicit storage/duplicate policies through {@link useCreateRepository};
 * the Web UI cannot authorize arbitrary host paths.
 *
 * Repository cards keep offline/error repositories visible for diagnosis but
 * refuse write and maintenance actions. Cloud credentials and import status
 * come through the Cloud public entry rather than being reimplemented here.
 *
 * ## Data
 *
 * {@link useRepositoryOptions} adapts the server repository list through
 * {@link normalizeRepositoryOptions}. {@link useRepositoryRoots} reads
 * admin-visible Storage Locations. {@link useRepositoryAssetCount} provides the
 * card-sized typed asset count.
 *
 * {@link useRepositoryScan} starts scans and stack detection.
 * {@link waitForRepositoryScan} follows a scan run to a terminal state before
 * repository-aware list/search queries are invalidated.
 * {@link RepositoryStatus} carries reachability; offline and error states are
 * not guessed from missing data. Consumers must use the root `index.ts`, which
 * is the complete cross-feature contract.
 *
 * @module
 */
import type { useCreateRepository } from "./api/useCreateRepository.ts";
import type { useRepositoryAssetCount } from "./api/useRepositoryAssetCount.ts";
import type { useRepositoryOptions } from "./api/useRepositoryOptions.ts";
import type { useRepositoryRoots } from "./api/useRepositoryRoots.ts";
import type { useRepositoryScan } from "./api/useRepositoryScan.ts";
import type { waitForRepositoryScan } from "./api/waitForRepositoryScan.ts";
import type BrowseScopeSelect from "./flows/browse-scope/BrowseScopeSelect.tsx";
import type { useBrowseScope } from "./flows/browse-scope/useBrowseScope.ts";
import type RepositoryGrid from "./flows/manage/RepositoryGrid.tsx";
import type { useWorkingRepository } from "./flows/working-repository/useWorkingRepository.ts";
import type { normalizeRepositoryOptions } from "./model/repositoryOptions.ts";
import type { RepositoryStatus } from "./types.ts";

export {};

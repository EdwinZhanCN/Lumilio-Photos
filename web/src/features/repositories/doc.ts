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
 *     GRID --> HOST["NativeHostActionModal"]
 *     HOST --> DESKTOP["local Desktop approval"]
 *     MONITOR["Server Monitor storage tab"] --> DIAGNOSTICS["useStorageDiagnostics"]
 *     MANAGE --> SCAN["scan / stack detection"]
 * ```
 *
 * {@link BrowseScopeSelect} is shared by read-oriented pages.
 * {@link RepositoryGrid} renders repositories as expandable rows grouped under
 * their Storage Location sections and owns the creation entry plus a
 * maintenance menu, but receives maintenance commands from Manage. Creation
 * is a four-step {@link AddRepositoryModal} wizard that selects an authorized
 * root, an explicit stable direct-child storage folder, and an immutable
 * storage strategy through {@link useCreateRepository}.
 * Every Web creation entry uses {@link StorageStrategyPicker}; later PATCH
 * mutation remains unavailable.
 *
 * {@link NativeHostActionModal} persists requests for a Desktop-native folder
 * grant and polls their durable status. The shared HTTP contract never accepts
 * a filesystem path or exposes the native approval nonce. Relocate-versus-copy
 * conflicts return user-facing resolutions and require an explicit copy
 * confirmation. The Web UI therefore owns the visible task while Desktop owns
 * the local chooser and final filesystem authority.
 * Standalone and Docker deployments use {@link RepositoryCandidateModal}
 * instead. It can only enumerate direct children of the configured default
 * Storage Location and opens a candidate by portable directory name.
 *
 * Repository rows keep unavailable repositories visible for diagnosis but
 * refuse write and maintenance actions. Parent Storage Location state takes
 * priority over child repository reachability, and activity is shown only once
 * storage is reachable. Cloud credentials and import status
 * come through the Cloud public entry rather than being reimplemented here.
 *
 * ## Data
 *
 * {@link useRepositoryOptions} adapts the server repository list through
 * {@link normalizeRepositoryOptions}. {@link useRepositoryRoots} reads
 * admin-visible Storage Locations. {@link useRepositoryAssetCount} provides the
 * row-sized typed asset count.
 * {@link useNativeHostCapability} gates Desktop handoff entry points and
 * {@link useNativeHostAction} resumes an outstanding task after refresh.
 * {@link useRepositoryCandidates} provides the bounded standalone directory
 * classification surface.
 * {@link useStorageDiagnostics}, lifecycle audit, and support-bundle queries
 * are exported for the admin-only Server Monitor storage view. Diagnostics
 * carry the owning Storage Location id for each repository so the monitor does
 * not infer filesystem hierarchy from path strings.
 *
 * {@link useRepositoryScan} starts scans and stack detection.
 * {@link waitForRepositoryScan} follows a scan run to a terminal state before
 * repository-aware list/search queries are invalidated.
 * {@link RepositoryReachability} carries storage availability while
 * {@link RepositoryActivity} carries current work; neither is guessed from
 * missing data. Consumers must use the root `index.ts`, which
 * is the complete cross-feature contract.
 *
 * @module
 */
import type { useCreateRepository } from "./api/useCreateRepository.ts";
import type { useNativeHostAction, useNativeHostCapability } from "./api/useNativeHostActions.ts";
import type { useRepositoryAssetCount } from "./api/useRepositoryAssetCount.ts";
import type { useRepositoryCandidates } from "./api/useRepositoryCandidates.ts";
import type { useRepositoryOptions } from "./api/useRepositoryOptions.ts";
import type { useRepositoryRoots } from "./api/useRepositoryRoots.ts";
import type { useRepositoryScan } from "./api/useRepositoryScan.ts";
import type { useStorageDiagnostics } from "./api/useStorageDiagnostics.ts";
import type { waitForRepositoryScan } from "./api/waitForRepositoryScan.ts";
import type { StorageStrategyPicker } from "./components/StorageStrategyPicker.tsx";
import type BrowseScopeSelect from "./flows/browse-scope/BrowseScopeSelect.tsx";
import type { useBrowseScope } from "./flows/browse-scope/useBrowseScope.ts";
import type RepositoryGrid from "./flows/manage/RepositoryGrid.tsx";
import type AddRepositoryModal from "./flows/manage/AddRepositoryModal.tsx";
import type NativeHostActionModal from "./flows/manage/NativeHostActionModal.tsx";
import type RepositoryCandidateModal from "./flows/manage/RepositoryCandidateModal.tsx";
import type { useWorkingRepository } from "./flows/working-repository/useWorkingRepository.ts";
import type { normalizeRepositoryOptions } from "./model/repositoryOptions.ts";
import type { RepositoryActivity, RepositoryReachability } from "./types.ts";

export {};

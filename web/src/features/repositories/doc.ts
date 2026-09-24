/**
 * # Repositories
 *
 * Repositories owns the shared repository option contract, browse scope,
 * concrete upload destination, and the admin Storage surface. Manage consumes
 * the non-admin list; admin Storage owns authorization, create/open, verify,
 * reconnect, detach, and native Desktop handoff.
 *
 * ## State
 *
 * Repository lists, Storage view, diagnostics, and cloud status are TanStack
 * Query server state. Browse and working repository ids are user-scoped
 * persisted preferences and are cleared by authentication reset.
 *
 * {@link useBrowseScope} permits an empty value meaning all repositories.
 * {@link useWorkingRepository} must resolve one concrete upload destination;
 * when no valid explicit choice exists it selects an upload-eligible primary
 * or first option. An explicit choice remains selected if it later becomes
 * unavailable so the application can explain the blocked target rather than
 * silently switch. Scan/detection id sets are request-local interaction inside
 * {@link useRepositoryScan}.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     BROWSE["browse pages"] --> BSCOPE["BrowseScopeSelect"]
 *     UPLOAD["Upload"] --> WORKING["useWorkingRepository"]
 *     MANAGE["Manage"] --> TARGETS["useRepositoryOptions"]
 *     STORAGE["admin Storage"] --> BROWSE["Browse: grouped table"]
 *     STORAGE --> HISTORY["History: LifecycleHistory"]
 *     BROWSE --> VIEW["useStorageView"]
 *     VIEW --> TABLE["RepositoryTable"]
 *     TABLE --> ROW["RepositoryRowActions"]
 *     TABLE --> INSPECTOR["RepositoryInspector"]
 *     INSPECTOR --> DIAGNOSTICS["useStorageDiagnostics"]
 *     VIEW --> CREATE["AddRepositoryModal"]
 *     VIEW --> HOST["NativeHostActionModal"]
 *     HOST --> DESKTOP["local Desktop approval"]
 *     VIEW --> CANDIDATES["RepositoryCandidateModal"]
 *     VIEW --> VERIFY["verify / history / cancel / duplicates / stacks"]
 * ```
 *
 * {@link BrowseScopeSelect} is shared by read-oriented pages. Manage consumes
 * {@link useRepositoryOptions} for its upload target only; it never renders an
 * administration surface.
 *
 * Admin Storage is {@link StoragePanelFlow}: one page with two views and a
 * structure that never changes shape with the data. Browse groups Repositories
 * by Storage Location; {@link RepositoryTable} carries capacity as a Repository
 * column together with the storage label that names where the figure was
 * measured. History is {@link LifecycleHistory} as a flat audit list with no
 * row expansion.
 * Row commands live in {@link RepositoryRowActions}, one menu per row, so an
 * action always names its Repository instead of relying on a page-level
 * selection.
 * {@link RepositoryInspector} opens inline beneath the selected row as one
 * level of plain text — a single column on narrow screens, two from `sm` up.
 * Capacity has exactly one visual form there, a sentence naming its storage
 * once, and the diagnostic facts sit in the same grid rather than a nested
 * disclosure.
 * {@link deriveStorageLabel} turns the Server's mount path into the storage
 * identity a reader recognizes; nothing is grouped by backing storage, because
 * one Storage Location can hold Repositories on different mounts.
 * Creation is a four-step {@link AddRepositoryModal} wizard that selects an
 * authorized Storage Location, an explicit stable direct-child storage folder,
 * and an immutable storage strategy through {@link useCreateRepository}.
 * Every Web creation entry uses {@link StorageStrategyPicker}; later PATCH
 * mutation remains unavailable.
 *
 * {@link NativeHostActionModal} persists requests for a Desktop-native folder
 * grant and polls their durable status. The shared HTTP contract never accepts
 * a filesystem path or exposes the native approval nonce. Relocate-versus-copy
 * conflicts return user-facing resolutions and require an explicit copy
 * confirmation. Standalone and Docker deployments use
 * {@link RepositoryCandidateModal} instead.
 *
 * Repository rows keep unavailable repositories visible for diagnosis.
 * Parent Storage Location status does not veto a healthy child. Cloud
 * credentials and import status come through the Cloud public entry rather
 * than being reimplemented here.
 *
 * ## Data
 *
 * {@link useRepositoryOptions} adapts `GET /api/v1/storage/targets` through
 * {@link normalizeRepositoryOptions}. {@link useStorageView} is the admin
 * aggregate. Both expose the discriminated {@link StorageEntity} presentation
 * contract: transport `name` becomes explicit `rawName`, while stable Storage
 * Location `kind` and Repository `role` determine reserved product names
 * through {@link getStorageEntityDisplayName}. UI consumers never infer
 * identity from seeded English names.
 * {@link buildStorageView} projects the admin read model without inventing
 * admission: reading and writing eligibility remain the Server's
 * `GET /api/v1/storage/targets` facts consumed through
 * {@link getRepositoryEffectiveState} for the non-admin surfaces.
 * {@link getRepositoryEffectiveState} maps closed admission reasons
 * (`offline`, `identity_error`, `read_only`, `low_space`, `paused`, `busy`,
 * `recovery_required`) and does not overlay Location health.
 * {@link useNativeHostCapability} gates Desktop handoff entry points and
 * {@link useNativeHostAction} resumes an outstanding task after refresh.
 * {@link useRepositoryCandidates} provides the bounded standalone directory
 * classification surface.
 * {@link useStorageDiagnostics} feeds the folded technical details and
 * {@link LifecycleHistory} renders the durable audit trail, both on the
 * admin-only Storage page. The support-bundle query downloads the diagnostic
 * archive from that page.
 *
 * {@link useRepositoryScan} starts verification and stack detection.
 * {@link useRepositoryVerificationCancel} cancels the latest active run, and
 * {@link VerificationHistoryModal} lists durable runs plus one-run detail.
 * Duplicate detection uses {@link useRepositoryDuplicateDetect} against
 * `/duplicates/detect` without importing Collections. A
 * verification mutation settles when the Server transaction returns its
 * immutable operation id and inserted/coalesced fact; it never waits for
 * background crawl or processing. Repository conflicts use the exact generated
 * Problem subtype for safe recovery facts. Scan and native-host terminal
 * states retain a Problem Reference, and their flows call
 * {@link localizeProblemReference} only when rendering. Consumers must use
 * the root `index.ts`, which is the complete cross-feature contract.
 *
 * @module
 */
import type { useCreateRepository } from "./api/useCreateRepository.ts";
import type { useNativeHostAction, useNativeHostCapability } from "./api/useNativeHostActions.ts";
import type { useRepositoryCandidates } from "./api/useRepositoryCandidates.ts";
import type { useRepositoryOptions } from "./api/useRepositoryOptions.ts";
import type { useRepositoryDuplicateDetect } from "./api/useRepositoryDuplicateDetect.ts";
import type { useRepositoryScan } from "./api/useRepositoryScan.ts";
import type { useRepositoryVerificationCancel } from "./api/useRepositoryVerifications.ts";
import type { useStorageDiagnostics } from "./api/useStorageDiagnostics.ts";
import type { useStorageView } from "./api/useStorageView.ts";
import type { StorageStrategyPicker } from "./components/StorageStrategyPicker.tsx";
import type BrowseScopeSelect from "./flows/browse-scope/BrowseScopeSelect.tsx";
import type { useBrowseScope } from "./flows/browse-scope/useBrowseScope.ts";
import type AddRepositoryModal from "./flows/storage-panel/AddRepositoryModal.tsx";
import type LifecycleHistory from "./flows/storage-panel/LifecycleHistory.tsx";
import type RepositoryInspector from "./flows/storage-panel/RepositoryInspector.tsx";
import type RepositoryRowActions from "./flows/storage-panel/RepositoryRowActions.tsx";
import type RepositoryTable from "./flows/storage-panel/RepositoryTable.tsx";
import type NativeHostActionModal from "./flows/storage-panel/NativeHostActionModal.tsx";
import type RepositoryCandidateModal from "./flows/storage-panel/RepositoryCandidateModal.tsx";
import type StoragePanelFlow from "./flows/storage-panel/StoragePanelFlow.tsx";
import type VerificationHistoryModal from "./flows/storage-panel/VerificationHistoryModal.tsx";
import type { buildStorageView } from "./flows/storage-panel/storageViewModel.ts";
import type { useWorkingRepository } from "./flows/working-repository/useWorkingRepository.ts";
import type { localizeProblemReference } from "../../lib/http-commons/problem.ts";
import type {
  getRepositoryEffectiveState,
  normalizeRepositoryOptions,
} from "./model/repositoryOptions.ts";
import type { getStorageEntityDisplayName } from "./model/storageEntities.ts";
import type { deriveStorageLabel } from "./model/storageLabels.ts";
import type { StorageEntity } from "./types.ts";

export {};

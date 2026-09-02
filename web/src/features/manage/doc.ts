/**
 * # Manage
 *
 * Manage owns the authenticated `/manage` operations surface. It composes
 * upload intake with Repository and catalog maintenance commands; durable
 * upload, repository, cloud, people, and duplicate state remains in those
 * domain features.
 *
 * ## State
 *
 * {@link Manage} has no durable state. Upload queue state comes from
 * {@link useUploadContext}. {@link RepositoryMaintenancePanel} keeps only
 * interaction state for the repository or job currently pending in each
 * action. Repository options and operation results remain TanStack Query
 * server state; Manage persists neither browse nor working-repository scope.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/manage"] --> PAGE["Manage"]
 *     PAGE --> UPLOAD["UnifiedUploadSection"]
 *     PAGE --> PANEL["RepositoryMaintenancePanel"]
 *     PANEL --> GRID["RepositoryGrid"]
 *     PANEL --> SCAN["scan / stack detection"]
 *     PANEL --> CATALOG["duplicates / people / locations"]
 *     PANEL --> CLOUD["cloud import"]
 * ```
 *
 * {@link Manage} composes the upload editor and
 * {@link RepositoryMaintenancePanel}. The panel supplies commands to the
 * repository-owned {@link RepositoryGrid}; rows do not import maintenance
 * domains themselves. Upload target choice remains inside
 * {@link UnifiedUploadSection}.
 *
 * Repository scans, stack detection, duplicate detection, location rebuild,
 * and cloud import are Repository-scoped. People clustering is catalog-wide
 * because identities can span repositories. Keeping these commands on Manage
 * prevents gallery pages from hiding expensive operational work.
 *
 * ## Data
 *
 * Repository lists come from {@link useRepositoryOptions}.
 * {@link useRepositoryScan} settles from the durable enqueue receipt and
 * invalidates repository-aware views; Repository rows own active-operation
 * polling through TanStack Query without extending mutation lifetime.
 * {@link useDetectDuplicates}, {@link useRebuildPeopleClusters}, and
 * {@link useStartRepositoryCloudImport} remain public commands of their owning
 * features.
 *
 * Repository rows use the typed asset-list count and
 * {@link useRepositoryCloudStatus}. A mutation acknowledgement may represent
 * queued background work; completion-aware hooks own any necessary polling and
 * invalidation rather than Manage copying job state.
 *
 * @module
 */
import type { useRepositoryCloudStatus, useStartRepositoryCloudImport } from "../cloud/index.ts";
import type { useDetectDuplicates } from "../collections/index.ts";
import type { useRebuildPeopleClusters } from "../people/index.ts";
import type {
  RepositoryGrid,
  useRepositoryOptions,
  useRepositoryScan,
} from "../repositories/index.ts";
import type { UnifiedUploadSection, useUploadContext } from "../upload/index.ts";
import type Manage from "./flows/overview/ManageFlow.tsx";
import type RepositoryMaintenancePanel from "./flows/overview/RepositoryMaintenancePanel.tsx";

export {};

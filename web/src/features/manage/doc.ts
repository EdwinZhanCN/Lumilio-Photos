/**
 * # Manage
 *
 * The Manage feature owns the authenticated `/manage` maintenance surface. It
 * composes upload intake with repository operations, so users have one place
 * to add new media and repair or refresh the repositories that already exist.
 * It does not own browse scope, asset gallery layout, album membership, people
 * editing, or system settings persistence; those concerns live in their
 * respective features and are invoked here through explicit hooks.
 *
 * ## State
 *
 * {@link Manage} is intentionally thin. It renders the page header, exposes
 * the supported-format modal, and mounts {@link UnifiedUploadSection} before
 * {@link RepositoryGrid}. The header reads {@link useUploadContext} only to
 * summarize the current queue; upload queue mutation remains in the upload
 * feature.
 *
 * Repository maintenance state is local to {@link RepositoryGrid}. Each action
 * tracks its own pending repository id or job state:
 *
 * - {@link useRepositoryScan} tracks rescan and stack-detection ids.
 * - {@link useDetectDuplicates} runs duplicate detection for one repository.
 * - {@link useRebuildPeopleClusters} starts the library-wide people rebuild.
 * - {@link useStartRepositoryCloudImport} starts cloud import for a bound
 *   repository.
 *
 * Creating repositories is also local to the grid modal. Local repositories
 * need only a display name; cloud repositories must pick a connected
 * credential from {@link useCloudCredentials}.
 *
 * ## Data
 *
 * Repository options come from {@link useIndexingRepositories}, the same
 * settings-owned source used by browse and working-repository pickers. Manage
 * reads from that source but does not persist repository selection
 * preferences.
 *
 * Repository cards show a scoped asset count through `/api/v1/assets/list` and
 * cloud status through {@link useRepositoryCloudStatus}. A repository scan
 * mutation only acknowledges a queued background job.
 * {@link waitForRepositoryScan} follows its scan run through running and
 * terminal backend states; repository-aware queries are invalidated only after
 * completion. Other maintenance mutations retain scoped invalidation behavior.
 *
 * The action scope is deliberately mixed:
 *
 * - Rescan, stack detection, duplicate detection, location rebuild, and cloud
 *   import are repository-scoped.
 * - People rebuild is library-wide because face clusters can span
 *   repositories.
 * - Upload target selection belongs to {@link UnifiedUploadSection} through the
 *   settings feature's working-repository hook, not to Manage itself.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/manage"] --> PAGE["Manage"]
 *     PAGE --> HEADER["ManageHeader"]
 *     PAGE --> UPLOAD["UnifiedUploadSection"]
 *     PAGE --> GRID["RepositoryGrid"]
 *     HEADER --> UCTX["useUploadContext"]
 *     GRID --> REPOS["useIndexingRepositories"]
 *     GRID --> SCAN["useRepositoryScan"]
 *     GRID --> DUP["useDetectDuplicates"]
 *     GRID --> PEOPLE["useRebuildPeopleClusters"]
 *     GRID --> CLOUD["cloud sync hooks"]
 *     GRID --> CARD["Repository cards"]
 *     CARD --> COUNT["repository asset count"]
 *     CARD --> STATUS["useRepositoryCloudStatus"]
 * ```
 *
 * {@link Manage} is therefore a composition route, not a data owner. It brings
 * together upload and repository maintenance but leaves each subsystem's
 * durable state in the feature that already owns it.
 *
 * ## Decisions
 *
 * Manage is the home for maintenance actions because the consequences are
 * repository- or library-wide. Gallery pages can show scoped media, but they
 * should not hide operations that rescan folders, rebuild locations, or launch
 * import jobs.
 *
 * Repository cards expose one busy state per action. This keeps a long-running
 * duplicate scan or cloud import from implying that unrelated repositories are
 * unavailable.
 *
 * People rebuild stays visible here even though People owns the domain model.
 * The rebuild is operational maintenance, not person editing, and it is
 * library-wide by design.
 *
 * @module
 */
import type Manage from "./routes/Manage.tsx";
import type RepositoryGrid from "./components/RepositoryGrid.tsx";
import type {
  useRepositoryScan,
  waitForRepositoryScan,
} from "./hooks/useRepositoryScan.ts";
import type UnifiedUploadSection from "@/features/upload/components/UnifiedUploadSection.tsx";
import type { useUploadContext } from "@/features/upload";
import type { useDetectDuplicates } from "@/features/collections/hooks/useDuplicates.ts";
import type { useRebuildPeopleClusters } from "@/features/people/hooks/usePeople.ts";
import type { useIndexingRepositories } from "@/features/settings/hooks/useAssetIndexing.ts";
import type {
  useCloudCredentials,
  useRepositoryCloudStatus,
  useStartRepositoryCloudImport,
} from "@/features/settings/hooks/useCloudSync.ts";

export {};

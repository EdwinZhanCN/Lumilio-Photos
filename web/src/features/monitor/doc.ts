/**
 * # Monitor
 *
 * Monitor owns the admin-only `/server-monitor` operational dashboard for
 * River queues, ML indexing coverage, rebuild commands, runtime capabilities,
 * and hierarchical storage health. It observes and triggers backend work but
 * does not define task enablement, queue semantics, or repository configuration.
 *
 * ## State
 *
 * {@link MonitorOverview} keeps the selected queue/ML/capabilities/storage tab in the
 * `tab` URL parameter. The ML view's optional repository scope is local to the
 * route and is not persisted as browse or upload preference.
 * {@link QueueSummaryList} keeps only expanded rows and transient copied status;
 * {@link MLMonitor} keeps its confirmation dialog and missing-only/full choice.
 *
 * Queue, capability, Lumen runtime, and indexing results remain TanStack Query server state.
 * The route checks the authenticated user before monitor queries render.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/server-monitor"] --> ADMIN["admin gate"]
 *     ADMIN --> TABS["queue / ML / capabilities / storage"]
 *     TABS --> QUEUE["StatMonitor + QueueSummaryList"]
 *     TABS --> ML["MLMonitor"]
 *     TABS --> CAP["CapabilitiesMonitor"]
 *     TABS --> STORAGE["StorageMonitor"]
 *     STORAGE --> HISTORY["LifecycleHistory"]
 *     ML --> REPOSITORY["optional repository scope"]
 *     ML --> REBUILD["task rebuild"]
 * ```
 *
 * {@link StatMonitor} and {@link QueueSummaryList} form the queue view.
 * {@link MLMonitor} combines coverage, repository options, and one confirmed
 * rebuild command. {@link CapabilitiesMonitor} is display-only; durable ML and
 * agent settings stay in Settings. {@link StorageMonitor} groups repositories
 * below their owning Storage Locations and exposes filesystem-writable capacity,
 * the server-owned safety reserve and resulting write budget, mount, risk, and
 * redacted support-bundle diagnostics in a fixed-height master-detail pane
 * whose tree and detail column scroll independently.
 * {@link LifecycleHistory} renders the durable lifecycle audit below the pane.
 *
 * ## Data
 *
 * Queue stats and summary endpoints poll every five seconds. Queue summaries
 * include bounded error samples suitable for copied diagnostics.
 * {@link useCapabilities} and {@link useLumenRuntime} poll every five seconds.
 * The public capability snapshot supplies de-sensitized task availability;
 * the administrator runtime snapshot supplies typed discovery-backend,
 * transport, compatibility, and node diagnostics. Refresh observes both
 * snapshots and never restarts or rescans discovery.
 *
 * {@link useAssetIndexingStats} polls repository-aware coverage every fifteen
 * seconds. {@link AssetIndexingStats} distinguishes photo and video totals and
 * semantic, video-semantic, BioCLIP, OCR, and face coverage.
 * {@link useRebuildAssetIndexes} can enqueue semantic, video-semantic, OCR, or
 * face work; BioCLIP is visible here but its album-scoped rebuild belongs to
 * Collections. {@link extractRebuildResponseData} interprets accepted/disabled
 * task results without inventing response shapes.
 *
 * @module
 */
import type {
  AssetIndexingStats,
  extractRebuildResponseData,
  useAssetIndexingStats,
  useRebuildAssetIndexes,
} from "./api/useAssetIndexing.ts";
import type { CapabilitiesMonitor } from "./flows/overview/CapabilitiesMonitor.tsx";
import type { LifecycleHistory } from "./flows/overview/LifecycleHistory.tsx";
import type { MLMonitor } from "./flows/overview/MLMonitor.tsx";
import type MonitorOverview from "./flows/overview/MonitorOverview.tsx";
import type { QueueSummaryList } from "./flows/overview/QueueSummaryList.tsx";
import type { StatMonitor } from "./flows/overview/StatMonitor.tsx";
import type { StorageMonitor } from "./flows/overview/StorageMonitor.tsx";
import type { useCapabilities } from "../../lib/capabilities/useCapabilities.ts";
import type { useLumenRuntime } from "./api/useLumenRuntime.ts";

export {};

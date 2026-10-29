/**
 * # Monitor
 *
 * Monitor owns the admin-only `/server-monitor` operational dashboard for
 * current Catalog processing, ML indexing coverage, rebuild commands, and
 * runtime capabilities. It observes and triggers backend work but does not
 * define task enablement, queue semantics, or repository configuration.
 *
 * ## State
 *
 * {@link MonitorOverview} keeps the selected queue/ML/capabilities tab in the
 * `tab` URL parameter, and {@link ProcessingMonitor} keeps the selected stage
 * in `stage` beside it. The ML view's optional repository scope is local to
 * the route and is not persisted as browse or upload preference.
 * {@link QueueSummaryList} keeps only expanded rows and transient copied status;
 * {@link MLMonitor} keeps its confirmation dialog and missing-only/full choice.
 * Hub selection and capability filtering remain local to the overview; nodes
 * are ordered by identity, four per orbit page.
 *
 * Processing, capability, Lumen runtime, and indexing results remain TanStack
 * Query server state. The route checks the authenticated user before monitor
 * queries render.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/server-monitor"] --> ADMIN["admin gate"]
 *     ADMIN --> TABS["queue / ML / capabilities"]
 *     TABS --> QUEUE["ProcessingMonitor"]
 *     QUEUE --> GRID["StageCard grid"]
 *     QUEUE --> PANEL["OverviewPanel / StageDetailPanel"]
 *     QUEUE --> DIAG["DiagnosticsDialog + QueueSummaryList"]
 *     TABS --> ML["MLMonitor"]
 *     TABS --> CAP["CapabilitiesMonitor"]
 *     ML --> REPOSITORY["optional repository scope"]
 *     ML --> REBUILD["task rebuild"]
 * ```
 *
 * {@link MonitorFrame} gives every tab one compact snapshot heading: the
 * successful-read timestamp, that tab's actions, and refresh. The route header
 * already names the tab and the primary visual already enumerates its subjects,
 * so the heading never repeats a title or tallies targets. A failed background
 * read retains cached facts with an explicit stale warning. Each tab retains
 * its own polling interval.
 *
 * {@link ProcessingMonitor} is a grid of identical {@link StageCard}s beside one
 * panel. Media cards (Import, Scan, Metadata, Thumbnails, Video, Analysis)
 * count files or Repositories; Catalog cards (Events, Places, Text search,
 * Backup) count updates or runs. Each card shows its status, the remaining
 * count in its unit, and one line: failures first, otherwise running work.
 * With nothing selected the panel is {@link OverviewPanel}: the Rive tray
 * driven by media in progress and six totals. Selecting a card turns it into
 * {@link StageDetailPanel}: four counts, facts (done, requesting sources,
 * oldest waiting, last activity), failed items with per-file and whole-stage
 * retry, then waiting items. Back or re-selecting the card restores the
 * overview. {@link DiagnosticsDialog} holds River delivery totals and queue
 * error samples, labelled as delivery attempts rather than file progress.
 * The Rive {@link ProcessingTray} appears once; reduced motion and runtime failure use static
 * geometry.
 *
 * ML uses equal hundred-cell coverage fields with one shared detail region.
 * A cell is approximately one percent, not a file; no-applicable-content is
 * separate from complete coverage. The five fields collapse to three, then two
 * columns at narrow container widths.
 * {@link CapabilityOrbit} shows actual Hub endpoints and their advertised
 * capabilities; layout encodes no hardware, latency, or performance topology.
 * Advertisements filter nodes but do not override public capability composition.
 * Agent configuration, backend and full node diagnostics remain expandable.
 * {@link MLMonitor} combines coverage, repository options, and one confirmed
 * rebuild command. It reports no job backlog, cross-tab links, or hidden
 * legend: files still awaiting analysis are a Processing lane. {@link CapabilitiesMonitor} is display-only; durable ML and
 * agent settings stay in Settings.
 *
 * Storage administration is not a Monitor tab. Storage Locations,
 * Repositories, capacity, verification, and the lifecycle audit belong to the
 * admin Storage route owned by `features/repositories`; Monitor reports
 * processing health only. Keeping storage out of Monitor leaves exactly one
 * authority for a storage fact.
 *
 * ## Data
 *
 * {@link useProcessingSummary} polls `/api/v1/admin/processing` every five
 * seconds. Every stage has the same fields — remaining, queued, running,
 * retrying, failed, and done for per-file stages — computed from Catalog
 * desired/applied facts; River supplies only running and retryable
 * deliveries. Reindex work is attributed as a source of Analysis, never a
 * stage. {@link useProcessingStageItems} pages a stage's failed or waiting
 * subjects with public reason codes; {@link useRetryProcessingStage} and
 * {@link useRetryProcessingItem} re-request failures through the Catalog.
 * {@link useProcessingDiagnostics} reads delivery diagnostics only while the
 * dialog is open.
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
import type { MonitorFrame } from "./flows/overview/MonitorFrame.tsx";
import type { CapabilityOrbit } from "./flows/overview/CapabilityOrbit.tsx";
import type { ProcessingTray } from "./modules/rive/ProcessingTray.tsx";
import type { MLMonitor } from "./flows/overview/MLMonitor.tsx";
import type MonitorOverview from "./flows/overview/MonitorOverview.tsx";
import type { QueueSummaryList } from "./flows/overview/QueueSummaryList.tsx";
import type { ProcessingMonitor } from "./flows/overview/ProcessingMonitor.tsx";
import type { StageCard } from "./flows/overview/StageCard.tsx";
import type { OverviewPanel, StageDetailPanel } from "./flows/overview/StagePanel.tsx";
import type { DiagnosticsDialog } from "./flows/overview/DiagnosticsDialog.tsx";
import type { useCapabilities } from "../../lib/capabilities/useCapabilities.ts";
import type {
  useProcessingDiagnostics,
  useProcessingStageItems,
  useProcessingSummary,
  useRetryProcessingItem,
  useRetryProcessingStage,
} from "./api/useProcessing.ts";
import type { useLumenRuntime } from "./api/useLumenRuntime.ts";

export {};

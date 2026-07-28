/**
 * # Upload
 *
 * Upload owns browser file intake, the in-session queue, hashing, duplicate
 * precheck, batch/chunk transport, ingest-job tracking, and global progress UI.
 * Manage chooses where the full editor appears; Upload owns how files reach a
 * concrete repository.
 *
 * ## State
 *
 * {@link UploadProvider} wraps the application with {@link UploadContext}.
 * {@link uploadReducer} is the queue-mutation boundary: {@link UploadState}
 * holds files, index-aligned preview URLs, and drag-over state, while
 * {@link UploadAction} defines additions, retries, preview replacement,
 * clearing, and drag transitions. Clearing or replacing files revokes obsolete
 * object URLs.
 *
 * {@link useUploadProcess} owns processing progress and active flags.
 * {@link useUploadProgressState} bridges per-file {@link FileUploadProgress}
 * into React state, while {@link runUploadProcess} coordinates the pipeline.
 * The queue is global but intentionally not persisted, so progress remains
 * visible across routes without pretending interrupted browser work is
 * resumable after a reload.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart LR
 *     PROVIDER["UploadProvider"] --> QUEUE["UnifiedUploadSection"]
 *     PROVIDER --> NAV["NavbarUploadQueue"]
 *     QUEUE --> PROCESS["useUploadProcess"]
 *     PROCESS --> HASH["useGenerateHashcode"]
 *     HASH --> PRECHECK["precheckUploads"]
 *     PRECHECK --> TRANSPORT["Batch or chunk transport"]
 *     TRANSPORT --> JOBS["waitForUploadJobs"]
 *     JOBS --> REFRESH["Refresh asset queries"]
 * ```
 *
 * {@link UnifiedUploadSection} validates files, edits the queue, chooses the
 * working repository, and starts processing. {@link NavbarUploadQueue} is a
 * compact global view over the same provider state and links back to Manage.
 *
 * {@link useGenerateHashcode} fingerprints files before
 * {@link precheckUploads}. Known files are marked duplicate and skip transport;
 * a failed precheck falls back to normal upload. Small files use
 * {@link useBatchUploadMutation}, large files use
 * {@link useChunkedUploadMutation}, and {@link waitForUploadJobs} follows
 * accepted ingest tasks to terminal backend state before asset queries refresh.
 *
 * ## Data
 *
 * {@link useWorkingRepository} resolves the concrete destination. “All
 * repositories” is valid for browsing but never for upload. {@link useUploadConfig}
 * reads server-owned chunk and concurrency limits; client values are temporary
 * resilience fallbacks while configuration is unavailable.
 *
 * The browser fingerprint mirrors backend BLAKE3 policy: full content through
 * 100 MiB, then a quick hash over little-endian size plus fixed first/last
 * chunks. Size accompanies the hash during duplicate checks. Per-file success,
 * duplicate, transport failure, and ingest failure remain distinct so retryable
 * `File` objects stay in the queue while completed files leave it.
 *
 * The feature root is the narrow public entry for application-level queue UI
 * and upload state. Hashing, transport, progress, and lifecycle helpers remain
 * internal to the feature or shared upload infrastructure.
 *
 * @module
 */
import type { useBatchUploadMutation, useChunkedUploadMutation } from "./api/useUploadMutations.ts";
import type { useUploadConfig } from "./api/useUploadQueries.ts";
import type UnifiedUploadSection from "./flows/intake/UnifiedUploadSection.tsx";
import type NavbarUploadQueue from "./flows/queue/NavbarUploadQueue.tsx";
import type { useGenerateHashcode } from "./modules/process/useGenerateHashcode.ts";
import type { FileUploadProgress, useUploadProcess } from "./modules/process/useUploadProcess.tsx";
import type { useUploadProgressState } from "./modules/process/progress.ts";
import type { runUploadProcess } from "./modules/process/runner.ts";
import type { UploadAction, UploadContext, UploadState } from "./state/context.ts";
import type { UploadProvider } from "./state/UploadProvider.tsx";
import type { uploadReducer } from "./state/reducer.ts";
import type { waitForUploadJobs } from "../../lib/upload/uploadLifecycle.ts";
import type { precheckUploads } from "../../lib/upload/uploadTransport.ts";
import type { useWorkingRepository } from "../repositories/index.ts";

export {};

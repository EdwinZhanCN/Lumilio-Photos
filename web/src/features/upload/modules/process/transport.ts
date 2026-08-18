import type { BatchUploadResponse, UploadPrecheckResult } from "@/lib/upload/types";
import { clearResumableSessionId, precheckUploads } from "@/lib/upload/uploadTransport";
import { waitForUploadJobs } from "@/lib/upload/uploadLifecycle";
import type { BatchUploadVariables, ChunkedUploadVariables } from "../../api/useUploadMutations.ts";
import { QUICK_FINGERPRINT_VERSION, QUICK_HASH_THRESHOLD } from "./config.ts";
import { createSemaphore } from "./concurrency.ts";
import { DUPLICATE_STATUS, isDuplicateResult, resolveResultStatus } from "./results.ts";
import type {
  FileUploadSession,
  UploadProcessResult,
  UploadProcessMessages,
  UploadProgressCallbacks,
  UploadRunResult,
  UploadTransportConfig,
} from "./types.ts";

export interface UploadTransportDependencies extends UploadProgressCallbacks {
  repositoryId?: string;
  config: UploadTransportConfig;
  messages: Pick<UploadProcessMessages, "noResult" | "processFailed" | "uploadFailed">;
  localizeProblem: (problem: unknown, fallback: string) => string;
  batchUpload: (variables: BatchUploadVariables) => Promise<BatchUploadResponse>;
  chunkedUpload: (variables: ChunkedUploadVariables) => Promise<BatchUploadResponse>;
}

export interface UploadTransport {
  uploadBatch: (sessions: FileUploadSession[]) => Promise<void>;
  uploadChunked: (session: FileUploadSession) => Promise<void>;
  waitForMaterialization: () => Promise<void>;
  getResult: () => UploadRunResult;
}

const transportSessionId = (session: FileUploadSession): string =>
  session.uploadSessionId ?? session.sessionId;

/** Prefer server session_id; fall back to file_name for older responses. */
export const matchBatchUploadResult = (
  result: UploadProcessResult,
  sessionsByUploadId: Map<string, FileUploadSession>,
  sessionsByFileName: Map<string, FileUploadSession[]>,
): FileUploadSession | undefined => {
  if (result.session_id) {
    const matched = sessionsByUploadId.get(result.session_id);
    if (matched) {
      sessionsByUploadId.delete(result.session_id);
      const bucket = sessionsByFileName.get(matched.file.name);
      const index = bucket?.indexOf(matched) ?? -1;
      if (bucket && index >= 0) bucket.splice(index, 1);
      return matched;
    }
  }
  const fromName = sessionsByFileName.get(result.file_name || "")?.shift();
  if (fromName) {
    sessionsByUploadId.delete(fromName.uploadSessionId ?? fromName.sessionId);
  }
  return fromName;
};

export const createUploadTransport = (
  dependencies: UploadTransportDependencies,
): UploadTransport => {
  const semaphore = createSemaphore(dependencies.config.maxConcurrentUploads);
  const results: UploadProcessResult[] = [];
  const resultSessions = new Map<UploadProcessResult, FileUploadSession>();
  const materializationSessions = new Map<number, FileUploadSession>();
  const materializationResults = new Map<number, UploadProcessResult>();

  const recordResult = (result: UploadProcessResult, session: FileUploadSession): void => {
    const normalizedResult =
      result.success && !isDuplicateResult(result) && !result.task_id
        ? {
            ...result,
            success: false,
            localError: dependencies.messages.noResult,
          }
        : !result.success
          ? {
              ...result,
              localError: dependencies.localizeProblem(
                result.problem,
                result.message || result.localError || dependencies.messages.uploadFailed,
              ),
            }
          : result;

    results.push(normalizedResult);
    resultSessions.set(normalizedResult, session);
    dependencies.updateFileProgress(session.sessionId, {
      status: resolveResultStatus(normalizedResult),
      progress: normalizedResult.success ? 100 : 0,
      error: normalizedResult.success
        ? undefined
        : normalizedResult.localError || dependencies.messages.uploadFailed,
    });

    if (
      normalizedResult.success &&
      (isDuplicateResult(normalizedResult) || Boolean(normalizedResult.task_id))
    ) {
      if (session.shouldUseChunks) {
        clearResumableSessionId(session.file, dependencies.repositoryId, session.hash);
      }
      if (normalizedResult.task_id) {
        materializationSessions.set(normalizedResult.task_id, session);
        materializationResults.set(normalizedResult.task_id, normalizedResult);
      }
    }
  };

  const recordFailure = (session: FileUploadSession, error: string): void => {
    const result: UploadProcessResult = {
      success: false,
      file_name: session.file.name,
      localError: error,
    };
    results.push(result);
    resultSessions.set(result, session);
    dependencies.updateFileProgress(session.sessionId, {
      status: "failed",
      progress: 0,
      error,
    });
  };

  const skipDuplicates = async (sessions: FileUploadSession[]): Promise<FileUploadSession[]> => {
    if (sessions.length === 0) return sessions;

    let precheckResults: UploadPrecheckResult[];
    try {
      const response = await precheckUploads(
        sessions.map((session) => ({
          hash: session.hash,
          size: session.file.size,
          is_quick: session.file.size > QUICK_HASH_THRESHOLD,
          fingerprint_version:
            session.file.size > QUICK_HASH_THRESHOLD ? QUICK_FINGERPRINT_VERSION : undefined,
        })),
        dependencies.repositoryId,
      );
      precheckResults = response.results ?? [];
    } catch {
      // Precheck is advisory. A failed request costs the optimization, not the upload.
      return sessions;
    }

    const pending: FileUploadSession[] = [];
    sessions.forEach((session, index) => {
      if (!precheckResults[index]?.duplicate) {
        pending.push(session);
        return;
      }

      recordResult(
        {
          success: true,
          file_name: session.file.name,
          content_hash: session.hash,
          status: DUPLICATE_STATUS,
        },
        session,
      );
    });
    return pending;
  };

  const uploadBatch = async (allSessions: FileUploadSession[]): Promise<void> => {
    await semaphore.acquire();
    let sessions = allSessions;
    try {
      sessions = await skipDuplicates(allSessions);
      if (sessions.length === 0) return;

      sessions.forEach((session) =>
        dependencies.updateFileProgress(session.sessionId, { status: "uploading" }),
      );

      const response = await dependencies.batchUpload({
        files: sessions.map((session) => ({
          file: session.file,
          sessionId: transportSessionId(session),
        })),
        repositoryId: dependencies.repositoryId,
        options: {
          onUploadProgress: (event) => {
            const progress = event.total ? Math.round((event.loaded * 100) / event.total) : 0;
            dependencies.setUploadProgress(progress);
            sessions.forEach((session) =>
              dependencies.updateFileProgress(session.sessionId, { progress }),
            );
          },
        },
      });

      const sessionsByUploadId = new Map(
        sessions.map((session) => [transportSessionId(session), session] as const),
      );
      const sessionsByFileName = new Map<string, FileUploadSession[]>();
      sessions.forEach((session) => {
        const bucket = sessionsByFileName.get(session.file.name);
        if (bucket) bucket.push(session);
        else sessionsByFileName.set(session.file.name, [session]);
      });

      (response.results ?? []).forEach((result) => {
        const match = matchBatchUploadResult(result, sessionsByUploadId, sessionsByFileName);
        if (match) recordResult(result, match);
      });

      sessionsByUploadId.forEach((session) => {
        recordFailure(session, dependencies.messages.noResult);
      });
    } catch (error) {
      const message = dependencies.localizeProblem(error, dependencies.messages.uploadFailed);
      sessions.forEach((session) => recordFailure(session, message));
    } finally {
      semaphore.release();
    }
  };

  const uploadChunked = async (candidate: FileUploadSession): Promise<void> => {
    await semaphore.acquire();
    try {
      const [session] = await skipDuplicates([candidate]);
      if (!session) return;

      dependencies.updateFileProgress(session.sessionId, { status: "uploading" });
      const response = await dependencies.chunkedUpload({
        file: session.file,
        sessionId: transportSessionId(session),
        hash: session.hash,
        repositoryId: dependencies.repositoryId,
        onProgress: (progress) => {
          dependencies.setUploadProgress(progress);
          dependencies.updateFileProgress(session.sessionId, { progress });
        },
        options: {
          maxConcurrent: dependencies.config.chunkConcurrency,
          chunkSize: dependencies.config.chunkSize,
        },
      });

      const result = response.results?.[0] ?? {
        success: false,
        file_name: session.file.name,
        localError: dependencies.messages.noResult,
      };
      recordResult(result, session);
    } catch (error) {
      recordFailure(
        candidate,
        dependencies.localizeProblem(error, dependencies.messages.uploadFailed),
      );
    } finally {
      semaphore.release();
    }
  };

  const waitForMaterialization = async (): Promise<void> => {
    const taskIds = Array.from(materializationSessions.keys());
    if (taskIds.length === 0) return;

    const settledTerminal = new Set<number>();
    try {
      await waitForUploadJobs(taskIds, {
        onUpdate: (job) => {
          if (!job.task_id) return;
          const session = materializationSessions.get(job.task_id);
          if (!session) return;
          if (!job.terminal) {
            dependencies.updateFileProgress(session.sessionId, {
              status: "processing",
              progress: 100,
            });
            return;
          }

          settledTerminal.add(job.task_id);
          const result = materializationResults.get(job.task_id);
          if (!job.success && result) {
            result.success = false;
            result.localError = dependencies.localizeProblem(
              job.problem,
              dependencies.messages.processFailed,
            );
          }
          dependencies.updateFileProgress(session.sessionId, {
            status: job.success ? "completed" : "failed",
            progress: job.success ? 100 : 0,
            error: job.success
              ? undefined
              : dependencies.localizeProblem(job.problem, dependencies.messages.processFailed),
          });
        },
      });
    } catch (error) {
      const message = dependencies.localizeProblem(error, dependencies.messages.processFailed);
      taskIds.forEach((taskId) => {
        if (settledTerminal.has(taskId)) return;
        const session = materializationSessions.get(taskId);
        const result = materializationResults.get(taskId);
        if (result) {
          result.success = false;
          result.localError = message;
        }
        if (session) {
          dependencies.updateFileProgress(session.sessionId, {
            status: "failed",
            progress: 0,
            error: message,
          });
        }
      });
    }
  };

  return {
    uploadBatch,
    uploadChunked,
    waitForMaterialization,
    getResult: () => ({ results, resultSessions }),
  };
};

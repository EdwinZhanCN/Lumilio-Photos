import type {
  BatchUploadResponse,
  BatchUploadResult,
  UploadPrecheckResult,
} from "@/lib/upload/types";
import { clearResumableSessionId, precheckUploads } from "@/lib/upload/uploadTransport";
import { waitForUploadJobs } from "@/lib/upload/uploadLifecycle";
import type { BatchUploadVariables, ChunkedUploadVariables } from "./useUploadMutations.ts";
import { QUICK_FINGERPRINT_VERSION, QUICK_HASH_THRESHOLD } from "./uploadProcessConfig.ts";
import { createSemaphore } from "./uploadProcessConcurrency.ts";
import {
  DUPLICATE_STATUS,
  isDuplicateResult,
  resolveResultStatus,
} from "./uploadProcessResults.ts";
import type {
  FileUploadSession,
  UploadProcessMessages,
  UploadProgressCallbacks,
  UploadRunResult,
  UploadTransportConfig,
} from "./uploadProcessTypes.ts";

export interface UploadTransportDependencies extends UploadProgressCallbacks {
  repositoryId?: string;
  config: UploadTransportConfig;
  messages: Pick<UploadProcessMessages, "noResult" | "processFailed" | "uploadFailed">;
  batchUpload: (variables: BatchUploadVariables) => Promise<BatchUploadResponse>;
  chunkedUpload: (variables: ChunkedUploadVariables) => Promise<BatchUploadResponse>;
}

export interface UploadTransport {
  uploadBatch: (sessions: FileUploadSession[]) => Promise<void>;
  uploadChunked: (session: FileUploadSession) => Promise<void>;
  waitForMaterialization: () => Promise<void>;
  getResult: () => UploadRunResult;
}

const getErrorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error ? error.message : fallback;

export const createUploadTransport = (
  dependencies: UploadTransportDependencies,
): UploadTransport => {
  const semaphore = createSemaphore(dependencies.config.maxConcurrentUploads);
  const results: BatchUploadResult[] = [];
  const resultSessions = new Map<BatchUploadResult, FileUploadSession>();
  const materializationSessions = new Map<number, FileUploadSession>();
  const materializationResults = new Map<number, BatchUploadResult>();

  const recordResult = (result: BatchUploadResult, session: FileUploadSession): void => {
    const normalizedResult =
      result.success && !isDuplicateResult(result) && !result.task_id
        ? {
            ...result,
            success: false,
            error: dependencies.messages.noResult,
          }
        : result;

    results.push(normalizedResult);
    resultSessions.set(normalizedResult, session);
    dependencies.updateFileProgress(session.sessionId, {
      status: resolveResultStatus(normalizedResult),
      progress: normalizedResult.success ? 100 : 0,
      error: normalizedResult.success
        ? undefined
        : normalizedResult.message || normalizedResult.error || dependencies.messages.uploadFailed,
    });

    if (
      normalizedResult.success &&
      (isDuplicateResult(normalizedResult) || Boolean(normalizedResult.task_id))
    ) {
      if (session.shouldUseChunks) {
        clearResumableSessionId(session.file, dependencies.repositoryId);
      }
      if (normalizedResult.task_id) {
        materializationSessions.set(normalizedResult.task_id, session);
        materializationResults.set(normalizedResult.task_id, normalizedResult);
      }
    }
  };

  const recordFailure = (session: FileUploadSession, error: string): void => {
    const result: BatchUploadResult = {
      success: false,
      file_name: session.file.name,
      error,
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
          sessionId: session.sessionId,
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

      const sessionsByFileName = new Map<string, FileUploadSession[]>();
      sessions.forEach((session) => {
        const bucket = sessionsByFileName.get(session.file.name);
        if (bucket) bucket.push(session);
        else sessionsByFileName.set(session.file.name, [session]);
      });

      (response.results ?? []).forEach((result) => {
        const match = sessionsByFileName.get(result.file_name || "")?.shift();
        if (match) recordResult(result, match);
      });

      sessionsByFileName.forEach((remaining) => {
        remaining.forEach((session) => recordFailure(session, dependencies.messages.noResult));
      });
    } catch (error) {
      const message = getErrorMessage(error, dependencies.messages.uploadFailed);
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
        sessionId: session.sessionId,
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
        error: dependencies.messages.noResult,
      };
      recordResult(result, session);
    } catch (error) {
      recordFailure(candidate, getErrorMessage(error, dependencies.messages.uploadFailed));
    } finally {
      semaphore.release();
    }
  };

  const waitForMaterialization = async (): Promise<void> => {
    const taskIds = Array.from(materializationSessions.keys());
    if (taskIds.length === 0) return;

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

          const result = materializationResults.get(job.task_id);
          if (!job.success && result) {
            result.success = false;
            result.error = job.error || dependencies.messages.processFailed;
          }
          dependencies.updateFileProgress(session.sessionId, {
            status: job.success ? "completed" : "failed",
            progress: job.success ? 100 : 0,
            error: job.success ? undefined : job.error || dependencies.messages.processFailed,
          });
        },
      });
    } catch (error) {
      const message = getErrorMessage(error, dependencies.messages.processFailed);
      taskIds.forEach((taskId) => {
        const session = materializationSessions.get(taskId);
        const result = materializationResults.get(taskId);
        if (result) {
          result.success = false;
          result.error = message;
        }
        if (session) {
          dependencies.updateFileProgress(session.sessionId, {
            status: "failed",
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

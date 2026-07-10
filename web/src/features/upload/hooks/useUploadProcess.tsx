import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useWorkingRepository } from "@/features/settings";
import { useI18n } from "@/lib/i18n";
import { HashcodeProgress, useGenerateHashcode } from "@/hooks/util-hooks/useGenerateHashcode";
import type { BatchUploadResult, UploadPrecheckResult } from "@/lib/upload/types";
import { generateSessionId, precheckUploads, shouldUseChunks } from "@/lib/upload/uploadTransport";
import { waitForUploadJobs } from "@/lib/upload/uploadLifecycle";
import {
  useBatchUploadMutation,
  useChunkedUploadMutation,
} from "@/features/upload/hooks/useUploadMutations";
import { useUploadConfig } from "@/features/upload/hooks/useUploadQueries";
import { getOptimalBatchSize, ProcessingPriority } from "@/lib/utils/smartBatchSizing.ts";

// Transport fallbacks used only while the server upload config is unavailable.
// The server endpoint is the source of truth for these values.
const FALLBACK_CHUNK_SIZE = 5 * 1024 * 1024;
const FALLBACK_MAX_CONCURRENT = 3;
const FALLBACK_MAX_IN_FLIGHT = 3;

// Server-side status marking content that already exists in the repository, so
// the upload was satisfied without transporting (or ingesting) the bytes.
const DUPLICATE_STATUS = "duplicate";

interface FailedFile {
  name: string;
  error: string;
  file: File;
}

interface ProcessResults {
  uploaded: string[];
  duplicates: string[];
  failed: FailedFile[];
}

type ProcessFilesFn = (files: FileList | File[]) => Promise<ProcessResults>;

// The server also reports duplicates the precheck missed, either because it was
// unreachable or because the content landed between precheck and transport.
const isDuplicateResult = (result: BatchUploadResult): boolean =>
  result.status === DUPLICATE_STATUS;

const resolveResultStatus = (result: BatchUploadResult): FileUploadProgress["status"] => {
  if (isDuplicateResult(result)) return "duplicate";
  if (!result.success || !result.task_id) return "failed";
  return "processing";
};

interface FileUploadSession {
  file: File;
  sessionId: string;
  hash: string;
  shouldUseChunks: boolean;
}

export interface FileUploadProgress {
  fileName: string;
  progress: number;
  status: "pending" | "uploading" | "processing" | "completed" | "duplicate" | "failed";
  sessionId: string;
  isChunked: boolean;
  error?: string;
}

export interface useUploadProcessReturn {
  processFiles: ProcessFilesFn;
  isUploading: boolean;
  isGeneratingHashCodes: boolean;
  resetStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: HashcodeProgress | null;
  fileProgress: FileUploadProgress[];
}

/**
 * Custom hook for handling file upload process with individual file progress tracking.
 * Optimized with pipelining and concurrency control.
 */
export function useUploadProcess(): useUploadProcessReturn {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { t } = useI18n();
  const { scopedRepositoryId } = useWorkingRepository();
  const [isUploading, setIsUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [fileProgress, setFileProgress] = useState<FileUploadProgress[]>([]);
  const batchUploadMutation = useBatchUploadMutation();
  const chunkedUploadMutation = useChunkedUploadMutation();
  const uploadConfigQuery = useUploadConfig();
  const serverUploadConfig = uploadConfigQuery.data;

  const {
    generateHashCodes,
    isGenerating: isGeneratingHashCodes,
    progress: hashcodeProgress,
  } = useGenerateHashcode();

  const updateFileProgress = useCallback(
    (sessionId: string, updates: Partial<FileUploadProgress>) => {
      setFileProgress((prev) =>
        prev.map((item) => (item.sessionId === sessionId ? { ...item, ...updates } : item)),
      );
    },
    [],
  );

  const invalidateAssetQueries = useCallback(() => {
    return queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (Array.isArray(key)) {
          const path = key[1];
          return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
        }
        return false;
      },
    });
  }, [queryClient]);

  const toPositiveInt = (value: number | undefined, fallback: number): number => {
    if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
      return Math.max(1, Math.floor(fallback));
    }
    return Math.max(1, Math.floor(value));
  };

  const processFiles: ProcessFilesFn = async (files) => {
    const fileArray = Array.from(files);
    if (fileArray.length === 0) return { uploaded: [], duplicates: [], failed: [] };

    setIsUploading(true);
    const results: BatchUploadResult[] = [];
    const materializationSessions = new Map<number, FileUploadSession>();
    const materializationResults = new Map<number, BatchUploadResult>();
    const resultSessions = new Map<BatchUploadResult, FileUploadSession>();
    const uploadTasks: Promise<void>[] = [];
    const smallFileBuffer: FileUploadSession[] = [];

    // Metrics
    const startTime = performance.now();
    let totalBytesHashed = 0;

    // Upload transport parameters are server-authoritative: the backend sizes
    // chunks and concurrency against its own memory. Fall back to fixed
    // defaults only while the server config request is in flight or failed.
    const maxConcurrentUploads = toPositiveInt(
      serverUploadConfig?.max_in_flight_requests,
      FALLBACK_MAX_IN_FLIGHT,
    );
    const effectiveChunkConcurrency = toPositiveInt(
      serverUploadConfig?.max_concurrent,
      FALLBACK_MAX_CONCURRENT,
    );
    const effectiveChunkSize = toPositiveInt(serverUploadConfig?.chunk_size, FALLBACK_CHUNK_SIZE);

    const semaphore = {
      count: maxConcurrentUploads,
      queue: [] as (() => void)[],
      async acquire() {
        if (this.count > 0) {
          this.count--;
          return;
        }
        await new Promise<void>((resolve) => this.queue.push(resolve));
      },
      release() {
        this.count++;
        if (this.queue.length > 0) {
          this.count--;
          const next = this.queue.shift();
          if (next) next();
        }
      },
    };

    const plannedSessions = fileArray.map((file) => ({
      file,
      sessionId: generateSessionId(),
      shouldUseChunks: shouldUseChunks(file),
    }));

    // Initialize progress state for all files with stable session identifiers.
    setFileProgress(
      plannedSessions.map((session) => ({
        fileName: session.file.name,
        progress: 0,
        status: "pending",
        sessionId: session.sessionId,
        isChunked: session.shouldUseChunks,
      })),
    );

    // Instant upload: ask the server which fingerprints it already holds and mark
    // those files as duplicates without transporting them. A precheck failure is
    // never fatal — it only costs us the saved bytes.
    const skipDuplicates = async (sessions: FileUploadSession[]): Promise<FileUploadSession[]> => {
      if (sessions.length === 0) return sessions;

      let precheckResults: UploadPrecheckResult[];
      try {
        const response = await precheckUploads(
          sessions.map((s) => ({ hash: s.hash, size: s.file.size })),
          scopedRepositoryId,
        );
        precheckResults = response.results ?? [];
      } catch {
        return sessions;
      }

      const pending: FileUploadSession[] = [];
      sessions.forEach((session, index) => {
        if (!precheckResults[index]?.duplicate) {
          pending.push(session);
          return;
        }
        results.push({
          success: true,
          file_name: session.file.name,
          content_hash: session.hash,
          status: DUPLICATE_STATUS,
        });
        updateFileProgress(session.sessionId, { status: "duplicate", progress: 100 });
      });
      return pending;
    };

    const uploadBatch = async (allSessions: FileUploadSession[]) => {
      const sessions = await skipDuplicates(allSessions);
      if (sessions.length === 0) return;

      await semaphore.acquire();
      try {
        sessions.forEach((s) =>
          updateFileProgress(s.sessionId, {
            status: "uploading",
          }),
        );

        const response = await batchUploadMutation.mutateAsync({
          files: sessions.map((s) => ({
            file: s.file,
            sessionId: s.sessionId,
          })),
          repositoryId: scopedRepositoryId,
          options: {
            onUploadProgress: (e) => {
              const p = e.total ? Math.round((e.loaded * 100) / e.total) : 0;
              setUploadProgress(p);
              sessions.forEach((s) => updateFileProgress(s.sessionId, { progress: p }));
            },
          },
        });

        const batchResults = response.results || [];
        results.push(...batchResults);

        const sessionsByFileName = new Map<string, FileUploadSession[]>();
        sessions.forEach((session) => {
          const bucket = sessionsByFileName.get(session.file.name);
          if (bucket) {
            bucket.push(session);
          } else {
            sessionsByFileName.set(session.file.name, [session]);
          }
        });

        batchResults.forEach((r) => {
          const fileName = r.file_name || "";
          const match = sessionsByFileName.get(fileName)?.shift();
          if (!match) {
            return;
          }
          if (r.success && !isDuplicateResult(r) && !r.task_id) {
            r.success = false;
            r.error = t("upload.UploadProcess.noResult");
          }
          resultSessions.set(r, match);
          updateFileProgress(match.sessionId, {
            status: resolveResultStatus(r),
            progress: r.success ? 100 : 0,
            error: r.success ? undefined : r.message || r.error,
          });
          if (r.task_id) {
            materializationSessions.set(r.task_id, match);
            materializationResults.set(r.task_id, r);
          }
        });
        sessionsByFileName.forEach((remaining) => {
          remaining.forEach((session) => {
            const result: BatchUploadResult = {
              success: false,
              file_name: session.file.name,
              error: t("upload.UploadProcess.noResult"),
            };
            results.push(result);
            resultSessions.set(result, session);
            updateFileProgress(session.sessionId, {
              status: "failed",
              progress: 0,
              error: result.error,
            });
          });
        });
      } catch (err: any) {
        sessions.forEach((s) => {
          const result = {
            success: false,
            file_name: s.file.name,
            error: err.message,
          };
          results.push(result);
          resultSessions.set(result, s);
          updateFileProgress(s.sessionId, {
            status: "failed",
            error: err.message,
          });
        });
      } finally {
        semaphore.release();
      }
    };

    const uploadChunked = async (candidate: FileUploadSession) => {
      const [session] = await skipDuplicates([candidate]);
      if (!session) return;

      await semaphore.acquire();
      try {
        updateFileProgress(session.sessionId, {
          status: "uploading",
        });
        const resp = await chunkedUploadMutation.mutateAsync({
          file: session.file,
          sessionId: session.sessionId,
          hash: session.hash,
          repositoryId: scopedRepositoryId,
          onProgress: (p) => {
            setUploadProgress(p);
            updateFileProgress(session.sessionId, { progress: p });
          },
          options: {
            maxConcurrent: effectiveChunkConcurrency,
            chunkSize: effectiveChunkSize,
          },
        });

        const result = resp.results?.[0] || {
          success: false,
          file_name: session.file.name,
          message: undefined,
          error: t("upload.UploadProcess.noResult"),
        };
        if (result.success && !isDuplicateResult(result) && !result.task_id) {
          result.success = false;
          result.error = t("upload.UploadProcess.noResult");
        }
        results.push(result);
        resultSessions.set(result, session);
        updateFileProgress(session.sessionId, {
          status: resolveResultStatus(result),
          progress: result.success ? 100 : 0,
          error: result.success ? undefined : result.message || result.error,
        });
        if (result.task_id) {
          materializationSessions.set(result.task_id, session);
          materializationResults.set(result.task_id, result);
        }
      } catch (err: any) {
        const result = {
          success: false,
          file_name: session.file.name,
          error: err.message,
        };
        results.push(result);
        resultSessions.set(result, session);
        updateFileProgress(session.sessionId, {
          status: "failed",
          error: err.message,
        });
      } finally {
        semaphore.release();
      }
    };

    try {
      // Determine optimal batch size for small files
      const optimalBatchSize = getOptimalBatchSize(
        "thumbnail",
        fileArray.length,
        ProcessingPriority.CRITICAL,
      );

      // Start hashing and pipeline the uploads
      await generateHashCodes(files, (hashResult) => {
        const session = plannedSessions[hashResult.index];
        totalBytesHashed += session.file.size;

        const sessionWithHash: FileUploadSession = {
          ...session,
          hash: hashResult.hash,
        };

        if (sessionWithHash.shouldUseChunks) {
          uploadTasks.push(uploadChunked(sessionWithHash));
        } else {
          smallFileBuffer.push(sessionWithHash);
          if (smallFileBuffer.length >= optimalBatchSize) {
            uploadTasks.push(uploadBatch([...smallFileBuffer]));
            smallFileBuffer.length = 0;
          }
        }
      });

      const endTime = performance.now();
      const durationSeconds = (endTime - startTime) / 1000;
      const speedMBps = totalBytesHashed / (1024 * 1024) / durationSeconds;
      console.log(
        `[Hash Metrics] Processed ${fileArray.length} files (${(totalBytesHashed / (1024 * 1024)).toFixed(2)} MB) in ${durationSeconds.toFixed(2)}s. Average speed: ${speedMBps.toFixed(2)} MB/s`,
      );

      // Upload remaining small files
      if (smallFileBuffer.length > 0) {
        uploadTasks.push(uploadBatch([...smallFileBuffer]));
      }

      await Promise.all(uploadTasks);

      const taskIds = Array.from(materializationSessions.keys());
      if (taskIds.length > 0) {
        try {
          await waitForUploadJobs(taskIds, {
            onUpdate: (job) => {
              if (!job.task_id) return;
              const session = materializationSessions.get(job.task_id);
              if (!session) return;
              if (!job.terminal) {
                updateFileProgress(session.sessionId, { status: "processing", progress: 100 });
                return;
              }
              const result = materializationResults.get(job.task_id);
              if (!job.success && result) {
                result.success = false;
                result.error = job.error || t("upload.UploadProcess.processFailed");
              }
              updateFileProgress(session.sessionId, {
                status: job.success ? "completed" : "failed",
                progress: job.success ? 100 : 0,
                error: job.success ? undefined : job.error,
              });
            },
          });
        } catch (error) {
          const message = error instanceof Error ? error.message : t("upload.UploadProcess.processFailed");
          taskIds.forEach((taskId) => {
            const session = materializationSessions.get(taskId);
            const result = materializationResults.get(taskId);
            if (result) {
              result.success = false;
              result.error = message;
            }
            if (session) updateFileProgress(session.sessionId, { status: "failed", error: message });
          });
        }
      }

      if (results.some((result) => result.success && !isDuplicateResult(result))) {
        await invalidateAssetQueries();
      }

      const duplicates = results.filter(isDuplicateResult).map((r) => r.file_name || "");
      const uploaded = results
        .filter((r) => r.success && !isDuplicateResult(r))
        .map((r) => r.file_name || "");
      const failed = results
        .filter((r) => !r.success)
        .map((r) => ({
          name: r.file_name || "Unknown",
          error: r.message || r.error || t("upload.UploadProcess.uploadFailed"),
          file: resultSessions.get(r)?.file ?? fileArray[0],
        }));

      if (failed.length > 0) {
        showMessage(
          "error",
          t("upload.UploadProcess.summaryPartial", {
            succeeded: uploaded.length,
            failed: failed.length,
          }),
        );
      } else if (duplicates.length > 0) {
        showMessage(
          uploaded.length > 0 ? "success" : "info",
          t(
            "upload.UploadProcess.summaryDuplicates",
            "Uploaded {{count}} files, skipped {{duplicates}} already in your library.",
            {
              count: uploaded.length,
              duplicates: duplicates.length,
            },
          ),
        );
      } else if (uploaded.length > 0) {
        showMessage(
          "success",
          t("upload.UploadProcess.summarySuccess", { count: uploaded.length }),
        );
      }

      return { uploaded, duplicates, failed };
    } catch (error: any) {
      await Promise.allSettled(uploadTasks);
      plannedSessions.forEach((session) =>
        updateFileProgress(session.sessionId, {
          status: "failed",
          error: error.message || t("upload.UploadProcess.processFailed"),
        }),
      );
      showMessage("error", error.message || t("upload.UploadProcess.processFailed"));
      return {
        uploaded: [],
        duplicates: [],
        failed: fileArray.map((f) => ({
          name: f.name,
          error: t("upload.UploadProcess.processFailed"),
          file: f,
        })),
      };
    } finally {
      setIsUploading(false);
      setUploadProgress(0);
    }
  };

  return {
    processFiles,
    isUploading,
    isGeneratingHashCodes,
    resetStatus: () => {
      setUploadProgress(0);
      setFileProgress([]);
    },
    uploadProgress,
    hashcodeProgress,
    fileProgress,
  };
}

import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useSettingsContext } from "@/features/settings";
import {
  HashcodeProgress,
  useGenerateHashcode,
} from "@/hooks/util-hooks/useGenerateHashcode";
import { uploadService, BatchUploadResult } from "@/services/uploadService";
import { globalPerformancePreferences } from "@/utils/performancePreferences";
import { getOptimalBatchSize, ProcessingPriority } from "@/utils/smartBatchSizing";

interface FailedFile {
  name: string;
  error: string;
}

interface ProcessResults {
  uploaded: string[];
  failed: FailedFile[];
}

type ProcessFilesFn = (files: FileList | File[]) => Promise<ProcessResults>;

interface FileUploadSession {
  file: File;
  sessionId: string;
  hash: string;
  shouldUseChunks: boolean;
}

export interface FileUploadProgress {
  fileName: string;
  progress: number;
  status: "pending" | "uploading" | "completed" | "failed";
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
  const { state: settings } = useSettingsContext();
  const [isUploading, setIsUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [fileProgress, setFileProgress] = useState<FileUploadProgress[]>([]);

  const {
    generateHashCodes,
    isGenerating: isGeneratingHashCodes,
    progress: hashcodeProgress,
  } = useGenerateHashcode();

  const updateFileProgress = useCallback((
    fileName: string,
    updates: Partial<FileUploadProgress>,
  ) => {
    setFileProgress((prev) =>
      prev.map((item) =>
        item.fileName === fileName ? { ...item, ...updates } : item,
      ),
    );
  }, []);

  const processFiles: ProcessFilesFn = async (files) => {
    const fileArray = Array.from(files);
    if (fileArray.length === 0) return { uploaded: [], failed: [] };

    setIsUploading(true);
    const results: BatchUploadResult[] = [];
    const uploadTasks: Promise<void>[] = [];
    const smallFileBuffer: FileUploadSession[] = [];

    // Metrics
    const startTime = performance.now();
    let totalBytesHashed = 0;

    // Concurrency control: dynamic based on performance preferences
    const maxConcurrentUploads = globalPerformancePreferences.getMaxConcurrentOperations() * 2; // Allow more for network I/O
    const semaphore = {
      count: maxConcurrentUploads,
      queue: [] as (() => void)[],
      async acquire() {
        if (this.count > 0) {
          this.count--;
          return;
        }
        await new Promise<void>(resolve => this.queue.push(resolve));
      },
      release() {
        this.count++;
        if (this.queue.length > 0) {
          this.count--;
          const next = this.queue.shift();
          if (next) next();
        }
      }
    };

    // Initialize progress state for all files
    setFileProgress(fileArray.map(file => ({
      fileName: file.name,
      progress: 0,
      status: "pending",
      sessionId: "",
      isChunked: uploadService.shouldUseChunks(file),
    })));

    const uploadBatch = async (sessions: FileUploadSession[]) => {
      await semaphore.acquire();
      try {
        sessions.forEach(s => updateFileProgress(s.file.name, { status: "uploading", sessionId: s.sessionId }));

        const response = await uploadService.batchUploadFiles(
          sessions.map(s => ({ file: s.file, sessionId: s.sessionId })),
          undefined,
          {
            onUploadProgress: (e) => {
              const p = e.total ? Math.round((e.loaded * 100) / e.total) : 0;
              setUploadProgress(p);
              sessions.forEach(s => updateFileProgress(s.file.name, { progress: p }));
            }
          }
        );

        const batchResults = response.data?.results || [];
        results.push(...batchResults);

        batchResults.forEach(r => {
          updateFileProgress(r.file_name || "", {
            status: r.success ? "completed" : "failed",
            progress: r.success ? 100 : 0,
            error: r.success ? undefined : (r.message || r.error)
          });
        });
      } catch (err: any) {
        sessions.forEach(s => {
          results.push({ success: false, file_name: s.file.name, error: err.message });
          updateFileProgress(s.file.name, { status: "failed", error: err.message });
        });
      } finally {
        semaphore.release();
      }
    };

    const uploadChunked = async (session: FileUploadSession) => {
      await semaphore.acquire();
      try {
        updateFileProgress(session.file.name, { status: "uploading", sessionId: session.sessionId });

        // Use performance preferences for chunk size if not explicitly set in UI settings
        const prefChunkSize = globalPerformancePreferences.getMemoryConstraintMultiplier() * 5 * 1024 * 1024; // Base 5MB
        const chunkSize = settings.ui.upload?.chunk_size_mb
          ? settings.ui.upload.chunk_size_mb * 1024 * 1024
          : prefChunkSize;

        const resp = await uploadService.uploadFileInChunks(
          session.file,
          session.sessionId,
          session.hash,
          undefined,
          undefined,
          (p) => {
            setUploadProgress(p);
            updateFileProgress(session.file.name, { progress: p });
          },
          {
            // Increase concurrency for HTTP/2 multiplexing, respect low power mode
            maxConcurrent: settings.ui.upload?.low_power_mode ? 2 : maxConcurrentUploads,
            chunkSize: chunkSize
          }
        );

        const result = resp.data?.results?.[0] || { success: false, file_name: session.file.name, error: "No result" };
        results.push(result);
        updateFileProgress(session.file.name, {
          status: result.success ? "completed" : "failed",
          progress: result.success ? 100 : 0
        });
      } catch (err: any) {
        results.push({ success: false, file_name: session.file.name, error: err.message });
        updateFileProgress(session.file.name, { status: "failed", error: err.message });
      } finally {
        semaphore.release();
      }
    };

    try {
      // Determine optimal batch size for small files
      const optimalBatchSize = getOptimalBatchSize("thumbnail", fileArray.length, ProcessingPriority.CRITICAL);

      // Start hashing and pipeline the uploads
      await generateHashCodes(files, (hashResult) => {
        const file = fileArray[hashResult.index];
        totalBytesHashed += file.size;

        const session: FileUploadSession = {
          file,
          hash: hashResult.hash,
          sessionId: uploadService.generateSessionId(),
          shouldUseChunks: uploadService.shouldUseChunks(file)
        };

        if (session.shouldUseChunks) {
          uploadTasks.push(uploadChunked(session));
        } else {
          smallFileBuffer.push(session);
          if (smallFileBuffer.length >= optimalBatchSize) {
            uploadTasks.push(uploadBatch([...smallFileBuffer]));
            smallFileBuffer.length = 0;
          }
        }
      });

      const endTime = performance.now();
      const durationSeconds = (endTime - startTime) / 1000;
      const speedMBps = (totalBytesHashed / (1024 * 1024)) / durationSeconds;
      console.log(`[Hash Metrics] Processed ${fileArray.length} files (${(totalBytesHashed / (1024 * 1024)).toFixed(2)} MB) in ${durationSeconds.toFixed(2)}s. Average speed: ${speedMBps.toFixed(2)} MB/s`);

      // Upload remaining small files
      if (smallFileBuffer.length > 0) {
        uploadTasks.push(uploadBatch([...smallFileBuffer]));
      }

      await Promise.all(uploadTasks);
      queryClient.invalidateQueries({ queryKey: ["userAssets"] });

      const uploaded = results.filter(r => r.success).map(r => r.file_name || "");
      const failed = results.filter(r => !r.success).map(r => ({
        name: r.file_name || "Unknown",
        error: r.message || r.error || "Upload failed"
      }));

      if (failed.length === 0 && uploaded.length > 0)
        showMessage("success", `Successfully uploaded ${uploaded.length} files.`);
      else if (uploaded.length > 0 || failed.length > 0)
        showMessage("error", `Upload complete: ${uploaded.length} succeeded, ${failed.length} failed.`);

      return { uploaded, failed };
    } catch (error: any) {
      showMessage("error", error.message || "Upload process failed");
      return { uploaded: [], failed: fileArray.map(f => ({ name: f.name, error: "Process failed" })) };
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

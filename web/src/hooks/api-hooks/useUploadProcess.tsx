import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { formatBytes } from "@/lib/utils/formatters.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import {
  HashcodeProgress,
  useGenerateHashcode,
} from "@/hooks/util-hooks/useGenerateHashcode";
import { uploadService, BatchUploadResult } from "@/services/uploadService";

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
 * @returns {useUploadProcessReturn}
 */
export function useUploadProcess(): useUploadProcessReturn {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [fileProgress, setFileProgress] = useState<FileUploadProgress[]>([]);

  const {
    generateHashCodes,
    isGenerating: isGeneratingHashCodes,
    progress: hashcodeProgress,
  } = useGenerateHashcode((metrics) => {
    const formattedSpeed = formatBytes(metrics.bytesPerSecond) + "/s";
    const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
    const formattedSize = formatBytes(metrics.totalSize);

    console.log(
      "hash",
      `Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}!`,
    );
  });

  const updateFileProgress = (
    fileName: string,
    updates: Partial<FileUploadProgress>,
  ) => {
    setFileProgress((prev) =>
      prev.map((item) =>
        item.fileName === fileName ? { ...item, ...updates } : item,
      ),
    );
  };

  const uploadMutation = useMutation({
    mutationFn: async (uploadSessions: FileUploadSession[]) => {
      // Initialize file progress tracking
      const initialProgress: FileUploadProgress[] = uploadSessions.map(
        (session) => ({
          fileName: session.file.name,
          progress: 0,
          status: "pending",
          sessionId: session.sessionId,
          isChunked: session.shouldUseChunks,
        }),
      );
      setFileProgress(initialProgress);

      // Separate files into chunked and non-chunked uploads
      const singleFiles = uploadSessions.filter(
        (session) => !session.shouldUseChunks,
      );
      const chunkedFiles = uploadSessions.filter(
        (session) => session.shouldUseChunks,
      );

      const results: BatchUploadResult[] = [];

      // Upload single files in batch with individual progress tracking
      if (singleFiles.length > 0) {
        const singleFileBatch = singleFiles.map((session) => ({
          file: session.file,
          sessionId: session.sessionId,
        }));

        // Update status for single files
        singleFiles.forEach((session) => {
          updateFileProgress(session.file.name, { status: "uploading" });
        });

        const singleResponse = await uploadService.batchUploadFiles(
          singleFileBatch,
          undefined,
          {
            onUploadProgress(progressEvent) {
              const progress = progressEvent.total
                ? Math.round((progressEvent.loaded * 100) / progressEvent.total)
                : 0;
              setUploadProgress(progress);

              // Update individual file progress for single files
              singleFiles.forEach((session) => {
                updateFileProgress(session.file.name, { progress });
              });
            },
          },
        );

        if (singleResponse.data?.data?.results) {
          results.push(...singleResponse.data.data.results);

          // Update status for completed single files
          singleResponse.data.data.results.forEach((result) => {
            if (result.success) {
              updateFileProgress(result.file_name || "", {
                status: "completed",
                progress: 100,
              });
            } else {
              updateFileProgress(result.file_name || "", {
                status: "failed",
                error: result.error || result.message,
              });
            }
          });
        }
      }

      // Upload chunked files individually with progress tracking
      for (const session of chunkedFiles) {
        try {
          updateFileProgress(session.file.name, { status: "uploading" });

          const chunkResponse = await uploadService.uploadFileInChunks(
            session.file,
            session.sessionId,
            undefined, // Use default chunk size
            undefined, // Use default repository
            (progress) => {
              // Update progress for individual chunked file
              setUploadProgress(progress);
              updateFileProgress(session.file.name, { progress });
            },
          );

          if (chunkResponse.data?.data?.results?.[0]) {
            const result = chunkResponse.data.data.results[0];
            results.push(result);

            if (result.success) {
              updateFileProgress(session.file.name, {
                status: "completed",
                progress: 100,
              });
            } else {
              updateFileProgress(session.file.name, {
                status: "failed",
                error: result.error || result.message,
              });
            }
          }
        } catch (error) {
          // Add failed result for chunked file
          results.push({
            success: false,
            file_name: session.file.name,
            error:
              error instanceof Error ? error.message : "Chunk upload failed",
            message: "Chunk upload failed",
          });

          updateFileProgress(session.file.name, {
            status: "failed",
            error:
              error instanceof Error ? error.message : "Chunk upload failed",
          });
        }
      }

      return { results };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["userAssets"] });
    },
    onSettled: () => {
      setUploadProgress(0);
    },
  });

  const processFiles: ProcessFilesFn = async (files) => {
    const fileArray = Array.from(files);
    if (fileArray.length === 0) {
      return { uploaded: [], failed: [] };
    }

    try {
      const hashResults = await generateHashCodes(files);
      if (!hashResults) throw new Error("Failed to generate hash codes");

      // Create upload sessions for each file
      const uploadSessions: FileUploadSession[] = fileArray.map(
        (file, index) => {
          const hashResult = hashResults?.find(
            (result: any) => result.index === index,
          );

          if (!hashResult?.hash) {
            throw new Error(`Failed to generate hash for file ${file.name}`);
          }

          return {
            file,
            hash: hashResult.hash,
            sessionId: uploadService.generateSessionId(),
            shouldUseChunks: uploadService.shouldUseChunks(file),
          };
        },
      );

      // Upload files using the new chunk strategy
      const response = await uploadMutation.mutateAsync(uploadSessions);

      const results: BatchUploadResult[] = response.results || [];

      const uploaded = results
        .filter((r) => r.success)
        .map((r) => r.file_name || "");

      const failed: FailedFile[] = results
        .filter((r) => !r.success)
        .map((r) => ({
          name: r.file_name || "Unknown file",
          error: r.message || r.error || "Upload failed",
        }));

      const totalUploaded = uploaded.length;
      const totalFailed = failed.length;

      // Show appropriate message based on upload results
      if (totalFailed === 0 && totalUploaded > 0) {
        // All successful
        const chunkedCount = uploadSessions.filter(
          (s) => s.shouldUseChunks,
        ).length;

        let message = `${totalUploaded} file(s) uploaded successfully.`;
        if (chunkedCount > 0) {
          message += ` (${chunkedCount} large files uploaded in chunks)`;
        }

        showMessage("success", message);
      } else if (totalUploaded === 0 && totalFailed > 0) {
        // All failed
        showMessage("error", `All ${totalFailed} file(s) failed to upload.`);
      } else if (totalUploaded > 0 && totalFailed > 0) {
        // Partial success
        showMessage(
          "error",
          `Upload complete: ${totalUploaded} succeeded, ${totalFailed} failed.`,
        );
      }

      return { uploaded, failed };
    } catch (error: any) {
      const errorMessage =
        error.response?.data?.message ||
        error.message ||
        "An unexpected error occurred.";
      showMessage("error", errorMessage);

      return {
        uploaded: [],
        failed: fileArray.map((f) => ({
          name: f.name,
          error: "Upload process failed",
        })),
      };
    }
  };

  return {
    processFiles,
    isUploading: uploadMutation.isPending,
    isGeneratingHashCodes,
    resetStatus: () => {
      uploadMutation.reset();
      setUploadProgress(0);
      setFileProgress([]);
    },
    uploadProgress,
    hashcodeProgress,
    fileProgress,
  };
}

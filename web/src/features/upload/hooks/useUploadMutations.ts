import { useMutation } from "@tanstack/react-query";
import {
  batchUploadFiles,
  uploadFile,
  uploadFileInChunks,
} from "@/lib/upload/uploadTransport";
import type {
  ApiResult,
  UploadResponse,
  BatchUploadResponse,
  UploadOptions,
  BatchUploadOptions,
  BatchUploadFile,
  ChunkedUploadOptions,
} from "@/lib/upload/types";

/**
 * Variables for single file upload mutations.
 */
export type UploadFileVariables = {
  /** The file to upload */
  file: File;
  /** Hash of the file content for deduplication */
  hash: string;
  /** Optional upload configuration options */
  options?: UploadOptions;
};

/**
 * Variables for batch upload mutations.
 */
export type BatchUploadVariables = {
  /** Array of files to upload in batch */
  files: BatchUploadFile[];
  /** Optional repository ID for the upload target */
  repositoryId?: string;
  /** Optional batch upload configuration options */
  options?: BatchUploadOptions;
};

/**
 * Variables for chunked upload mutations.
 */
export type ChunkedUploadVariables = {
  /** The file to upload in chunks */
  file: File;
  /** Unique session identifier for the chunked upload */
  sessionId: string;
  /** Hash of the file content for deduplication */
  hash: string;
  /** Optional custom chunk size in bytes */
  chunkSize?: number;
  /** Optional repository ID for the upload target */
  repositoryId?: string;
  /** Optional progress callback function */
  onProgress?: (progress: number) => void;
  /** Optional chunked upload configuration options */
  options?: ChunkedUploadOptions;
};

/**
 * React Query mutation hook for uploading a single file.
 * 
 * @returns Mutation object for single file uploads
 * 
 * @example
 * ```typescript
 * const uploadMutation = useUploadFileMutation();
 * 
 * uploadMutation.mutate({
 *   file: selectedFile,
 *   hash: fileHash,
 *   options: { overwrite: true }
 * });
 * ```
 */
export const useUploadFileMutation = () =>
  useMutation<ApiResult<UploadResponse>, Error, UploadFileVariables>({
    mutationFn: ({ file, hash, options }) => uploadFile(file, hash, options),
  });

/**
 * React Query mutation hook for uploading multiple files in batch.
 * 
 * @returns Mutation object for batch file uploads
 * 
 * @example
 * ```typescript
 * const batchUploadMutation = useBatchUploadMutation();
 * 
 * batchUploadMutation.mutate({
 *   files: batchFiles,
 *   repositoryId: 'repo-123',
 *   options: { parallel: true }
 * });
 * ```
 */
export const useBatchUploadMutation = () =>
  useMutation<ApiResult<BatchUploadResponse>, Error, BatchUploadVariables>({
    mutationFn: ({ files, repositoryId, options }) =>
      batchUploadFiles(files, repositoryId, options),
  });

/**
 * React Query mutation hook for uploading large files in chunks.
 * 
 * This is ideal for large files that need to be uploaded in smaller pieces
 * to handle network interruptions and provide progress updates.
 * 
 * @returns Mutation object for chunked file uploads
 * 
 * @example
 * ```typescript
 * const chunkedUploadMutation = useChunkedUploadMutation();
 * 
 * chunkedUploadMutation.mutate({
 *   file: largeFile,
 *   sessionId: 'session-123',
 *   hash: fileHash,
 *   chunkSize: 1024 * 1024, // 1MB chunks
 *   onProgress: (progress) => console.log(`Upload: ${progress}%`)
 * });
 * ```
 */
export const useChunkedUploadMutation = () =>
  useMutation<ApiResult<BatchUploadResponse>, Error, ChunkedUploadVariables>({
    mutationFn: ({
      file,
      sessionId,
      hash,
      chunkSize,
      repositoryId,
      onProgress,
      options,
    }) =>
      uploadFileInChunks(
        file,
        sessionId,
        hash,
        chunkSize,
        repositoryId,
        onProgress,
        options,
      ),
  });

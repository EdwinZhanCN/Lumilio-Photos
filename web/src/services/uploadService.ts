import api from "@/lib/http-commons/api.ts";
import type { AxiosRequestConfig, AxiosResponse } from "axios";
import type { components } from "@/lib/http-commons/schema.d.ts";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];

/**
 * Standard API response wrapper
 */
export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

/**
 * Single file upload response
 */
export type UploadResponse = Schemas["dto.UploadResponseDTO"];

/**
 * Batch upload response containing results array
 */
export type BatchUploadResponse = Schemas["dto.BatchUploadResponseDTO"];

/**
 * Individual file result within batch upload
 */
export type BatchUploadResult = Schemas["dto.BatchUploadResultDTO"];

/**
 * Upload configuration response
 */
export type UploadConfigResponse = Schemas["dto.UploadConfigResponseDTO"];

/**
 * Upload progress response
 */
export type UploadProgressResponse = Schemas["dto.UploadProgressResponseDTO"];

/**
 * Session progress information
 */
export type SessionProgress = Schemas["dto.SessionProgressDTO"];

// ============================================================================
// Upload Service
// ============================================================================

export const uploadService = {
  /**
   * Upload a single file to the server
   * @param file - The file to upload
   * @param hash - Unique file identifier (BLAKE3 hash)
   * @param config - Optional Axios config (e.g., onUploadProgress)
   * @returns A promise resolving to an Axios response with UploadResponse
   */
  uploadFile: async (
    file: File,
    hash: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<UploadResponse>>> => {
    const formData = new FormData();
    formData.append("file", file);

    return api.post<ApiResult<UploadResponse>>("/api/v1/assets", formData, {
      ...config,
      headers: {
        "Content-Type": "multipart/form-data",
        "X-Content-Hash": hash,
        ...config?.headers,
      },
    });
  },

  /**
   * Batch upload multiple files with unified chunk support
   * Each file's field name should follow format: single_{session_id} for single files
   * or chunk_{session_id}_{index}_{total} for chunks
   * @param files - Array of file objects with session IDs and optional chunk info
   * @param repositoryId - Optional repository UUID
   * @param config - Optional Axios config
   * @returns A promise resolving to batch upload results
   */
  batchUploadFiles: async (
    files: Array<{
      file: Blob;
      fileName?: string;
      sessionId: string;
      isChunk?: boolean;
      chunkIndex?: number;
      totalChunks?: number;
    }>,
    repositoryId?: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<BatchUploadResponse>>> => {
    const formData = new FormData();

    // Add repository ID if provided
    if (repositoryId) {
      formData.append("repository_id", repositoryId);
    }

    // Process each file with proper field naming
    files.forEach((fileObj) => {
      let fieldName: string;

      if (
        fileObj.isChunk &&
        fileObj.chunkIndex !== undefined &&
        fileObj.totalChunks !== undefined
      ) {
        // Chunk format: chunk_{id}_{index}_{total}
        fieldName = `chunk_${fileObj.sessionId}_${fileObj.chunkIndex}_${fileObj.totalChunks}`;
      } else {
        // Single file format: single_{id}
        fieldName = `single_${fileObj.sessionId}`;
      }

      const filename =
        fileObj.fileName || (fileObj.file as File).name || "upload";
      formData.append(fieldName, fileObj.file, filename);
    });

    return api.post<ApiResult<BatchUploadResponse>>(
      "/api/v1/assets/batch",
      formData,
      {
        ...config,
        headers: {
          "Content-Type": "multipart/form-data",
          ...config?.headers,
        },
      },
    );
  },

  /**
   * Upload a single file as chunks
   * @param file - The file to upload in chunks
   * @param sessionId - Unique session identifier
   * @param chunkSize - Size of each chunk in bytes
   * @param repositoryId - Optional repository UUID
   * @param onProgress - Optional progress callback
   * @returns A promise resolving to batch upload results
   */
  uploadFileInChunks: async (
    file: File,
    sessionId: string,
    chunkSize: number = 24 * 1024 * 1024, // 24MB default to reduce chunk count
    repositoryId?: string,
    onProgress?: (progress: number) => void,
    options?: { maxConcurrent?: number; chunkSize?: number },
  ): Promise<AxiosResponse<ApiResult<BatchUploadResponse>>> => {
    const effectiveChunkSize = options?.chunkSize ?? chunkSize;
    const totalChunks = Math.ceil(file.size / effectiveChunkSize);
    const uploadPromises: Array<
      Promise<AxiosResponse<ApiResult<BatchUploadResponse>>>
    > = [];
    const maxConcurrent = options?.maxConcurrent ?? 2; // Lower concurrency for low-power mode

    // Upload chunks sequentially with concurrency control
    for (
      let chunkIndex = 0;
      chunkIndex < totalChunks;
      chunkIndex += maxConcurrent
    ) {
      const chunkBatch = [];

      for (let i = 0; i < maxConcurrent && chunkIndex + i < totalChunks; i++) {
        const currentChunkIndex = chunkIndex + i;
        const start = currentChunkIndex * effectiveChunkSize;
        const end = Math.min(start + effectiveChunkSize, file.size);
        const chunk = file.slice(start, end);

        chunkBatch.push({
          file: chunk,
          fileName: file.name,
          sessionId,
          isChunk: true,
          chunkIndex: currentChunkIndex,
          totalChunks,
        });
      }

      // Upload this batch of chunks
      const response = await uploadService.batchUploadFiles(
        chunkBatch,
        repositoryId,
      );

      // Update progress
      if (onProgress) {
        const progress = Math.min(
          ((chunkIndex + chunkBatch.length) / totalChunks) * 100,
          100,
        );
        onProgress(progress);
      }

      uploadPromises.push(Promise.resolve(response));
    }

    // Return the last response
    return uploadPromises[uploadPromises.length - 1];
  },

  /**
   * Get upload configuration including chunk size and concurrency limits
   * @returns A promise resolving to upload configuration
   */
  getUploadConfig: async (): Promise<
    AxiosResponse<ApiResult<UploadConfigResponse>>
  > => {
    return api.get<ApiResult<UploadConfigResponse>>(
      "/api/v1/assets/batch/config",
    );
  },

  /**
   * Get upload progress for specific sessions
   * @param sessionIds - Optional comma-separated session IDs
   * @returns A promise resolving to upload progress information
   */
  getUploadProgress: async (
    sessionIds?: string,
  ): Promise<AxiosResponse<ApiResult<UploadProgressResponse>>> => {
    const params = sessionIds ? { session_ids: sessionIds } : undefined;
    return api.get<ApiResult<UploadProgressResponse>>(
      "/api/v1/assets/batch/progress",
      { params },
    );
  },

  /**
   * Generate a unique session ID for file uploads
   * @returns A unique session identifier
   */
  generateSessionId: (): string => {
    return crypto.randomUUID();
  },

  /**
   * Determine if a file should be uploaded in chunks based on size
   * @param file - The file to check
   * @param threshold - Size threshold in bytes (default: 10MB)
   * @returns Whether the file should be chunked
   */
  shouldUseChunks: (
    file: File,
    threshold: number = 10 * 1024 * 1024,
  ): boolean => {
    return file.size > threshold;
  },
};

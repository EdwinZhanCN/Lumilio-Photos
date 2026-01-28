// src/services/uploadService.ts

import { getToken } from "@/lib/http-commons/api";
import type { components } from "@/lib/http-commons/schema.d.ts";
import client from "@/lib/http-commons/client";

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

// Base URL for uploads
const baseURL = import.meta.env.VITE_API_URL || "http://localhost:8080";

/**
 * Options for upload progress tracking
 */
export interface UploadOptions {
  onUploadProgress?: (progress: { loaded: number; total: number; percent: number }) => void;
  signal?: AbortSignal;
}

// ============================================================================
// Upload Service (using native fetch for multipart/form-data)
// ============================================================================

export const uploadService = {
  /**
   * Upload a single file to the server
   * @param file - The file to upload
   * @param hash - Unique file identifier (BLAKE3 hash)
   * @param options - Optional config (progress callback, abort signal)
   * @returns A promise resolving to upload response
   */
  uploadFile: async (
    file: File,
    hash: string,
    options?: UploadOptions,
  ): Promise<ApiResult<UploadResponse>> => {
    const formData = new FormData();
    formData.append("file", file);

    const headers: HeadersInit = {
      "X-Content-Hash": hash,
    };

    const token = getToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    // Use XMLHttpRequest for progress tracking if callback provided
    if (options?.onUploadProgress) {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", `${baseURL}/assets`);

        // Set headers
        Object.entries(headers).forEach(([key, value]) => {
          xhr.setRequestHeader(key, value);
        });

        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable && options.onUploadProgress) {
            options.onUploadProgress({
              loaded: event.loaded,
              total: event.total,
              percent: Math.round((event.loaded / event.total) * 100),
            });
          }
        };

        xhr.onload = () => {
          try {
            const response = JSON.parse(xhr.responseText);
            resolve(response);
          } catch {
            reject(new Error("Failed to parse response"));
          }
        };

        xhr.onerror = () => reject(new Error("Upload failed"));

        if (options.signal) {
          options.signal.addEventListener("abort", () => xhr.abort());
        }

        xhr.send(formData);
      });
    }

    // Use fetch for simple uploads without progress
    const response = await fetch(`${baseURL}/assets`, {
      method: "POST",
      headers,
      body: formData,
      signal: options?.signal,
    });

    return response.json();
  },

  /**
   * Batch upload multiple files with unified chunk support
   * Each file's field name should follow format: single_{session_id} for single files
   * or chunk_{session_id}_{index}_{total} for chunks
   * @param files - Array of file objects with session IDs and optional chunk info
   * @param repositoryId - Optional repository UUID
   * @param options - Optional config (progress callback, abort signal)
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
    options?: UploadOptions & { contentHash?: string },
  ): Promise<ApiResult<BatchUploadResponse>> => {
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

    const headers: HeadersInit = {};

    if (options?.contentHash) {
      headers["X-Content-Hash"] = options.contentHash;
    }

    const token = getToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    // Use XMLHttpRequest for progress tracking if callback provided
    if (options?.onUploadProgress) {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", `${baseURL}/assets/batch`);

        // Set headers
        Object.entries(headers).forEach(([key, value]) => {
          xhr.setRequestHeader(key, value);
        });

        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable && options.onUploadProgress) {
            options.onUploadProgress({
              loaded: event.loaded,
              total: event.total,
              percent: Math.round((event.loaded / event.total) * 100),
            });
          }
        };

        xhr.onload = () => {
          try {
            const response = JSON.parse(xhr.responseText);
            resolve(response);
          } catch {
            reject(new Error("Failed to parse response"));
          }
        };

        xhr.onerror = () => reject(new Error("Batch upload failed"));

        if (options.signal) {
          options.signal.addEventListener("abort", () => xhr.abort());
        }

        xhr.send(formData);
      });
    }

    // Use fetch for simple uploads without progress
    const response = await fetch(`${baseURL}/assets/batch`, {
      method: "POST",
      headers,
      body: formData,
      signal: options?.signal,
    });

    return response.json();
  },

  /**
   * Upload a single file as chunks
   * @param file - The file to upload in chunks
   * @param sessionId - Unique session identifier
   * @param hash - The BLAKE3 hash of the file
   * @param chunkSize - Size of each chunk in bytes
   * @param repositoryId - Optional repository UUID
   * @param onProgress - Optional progress callback
   * @returns A promise resolving to batch upload results
   */
  uploadFileInChunks: async (
    file: File,
    sessionId: string,
    hash: string,
    chunkSize: number = 24 * 1024 * 1024,
    repositoryId?: string,
    onProgress?: (progress: number) => void,
    options?: { maxConcurrent?: number; chunkSize?: number },
  ): Promise<ApiResult<BatchUploadResponse>> => {
    const effectiveChunkSize = options?.chunkSize ?? chunkSize;
    const totalChunks = Math.ceil(file.size / effectiveChunkSize);
    // Increased default concurrency for HTTP/2 multiplexing
    const maxConcurrent = options?.maxConcurrent ?? 6;
    let lastResponse: ApiResult<BatchUploadResponse> | null = null;

    // Helper to upload a single chunk
    const uploadChunk = async (chunkIndex: number) => {
      const start = chunkIndex * effectiveChunkSize;
      const end = Math.min(start + effectiveChunkSize, file.size);
      const chunk = file.slice(start, end);

      return uploadService.batchUploadFiles(
        [{
          file: chunk,
          fileName: file.name,
          sessionId,
          isChunk: true,
          chunkIndex,
          totalChunks,
        }],
        repositoryId,
        {
          contentHash: hash,
        }
      );
    };

    // Use a semaphore-like approach to limit concurrency
    let activeUploads = 0;
    let nextChunkIndex = 0;
    let completedChunks = 0;

    return new Promise((resolve, reject) => {
      const processNext = () => {
        if (nextChunkIndex >= totalChunks) {
          if (activeUploads === 0 && lastResponse) {
            resolve(lastResponse);
          }
          return;
        }

        while (activeUploads < maxConcurrent && nextChunkIndex < totalChunks) {
          const currentIndex = nextChunkIndex++;
          activeUploads++;

          uploadChunk(currentIndex)
            .then(response => {
              lastResponse = response;
              completedChunks++;
              activeUploads--;

              if (onProgress) {
                const progress = Math.min((completedChunks / totalChunks) * 100, 100);
                onProgress(progress);
              }

              processNext();
            })
            .catch(error => {
              reject(error);
            });
        }
      };

      processNext();
    });
  },

  /**
   * Get upload configuration including chunk size and concurrency limits
   * @returns A promise resolving to upload configuration
   */
  getUploadConfig: async (): Promise<UploadConfigResponse | undefined> => {
    const { data } = await client.GET("/api/v1/assets/batch/config", {});
    return data?.data as UploadConfigResponse | undefined;
  },

  /**
   * Get upload progress for specific sessions
   * @param sessionIds - Optional comma-separated session IDs
   * @returns A promise resolving to upload progress information
   */
  getUploadProgress: async (
    sessionIds?: string,
  ): Promise<UploadProgressResponse | undefined> => {
    const { data } = await client.GET("/api/v1/assets/batch/progress", {
      params: { query: sessionIds ? { session_ids: sessionIds } : undefined },
    });
    return data?.data as UploadProgressResponse | undefined;
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

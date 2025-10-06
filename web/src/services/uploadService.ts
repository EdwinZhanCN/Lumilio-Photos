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
export type UploadResponse = Schemas["handler.UploadResponse"];

/**
 * Batch upload response containing results array
 */
export type BatchUploadResponse = Schemas["handler.BatchUploadResponse"];

/**
 * Individual file result within batch upload
 */
export type BatchUploadResult = Schemas["handler.BatchUploadResult"];

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
   * Batch upload multiple files.
   * Each file's field name should be its BLAKE3 content hash.
   * @param files - Array of file objects with their computed hashes
   * @param config - Optional Axios config
   * @returns A promise resolving to batch upload results
   */
  batchUploadFiles: async (
    files: { file: File; hash: string }[],
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<BatchUploadResponse>>> => {
    const formData = new FormData();

    files.forEach((fileObj) => {
      formData.append(fileObj.hash, fileObj.file, fileObj.file.name);
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
};

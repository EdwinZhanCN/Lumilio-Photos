import api from "@/lib/api";
import { AxiosRequestConfig, AxiosResponse } from "axios";
// Response interfaces based on swagger.yaml
export interface UploadResponse {
  content_hash: string;
  file_name: string;
  message: string;
  size: number;
  status: string;
  task_id: string;
}

// 根据您的 Go 后端和响应示例，这是单个结果的精确类型
export interface BatchUploadResult {
  success: boolean;
  file_name: string;
  content_hash: string;
  task_id: string;
  status: string;
  size: number;
  message: string;
  error?: string; // 失败时可能有
}

// 这是响应体中 `data` 字段的内容
export interface BatchUploadData {
  results: BatchUploadResult[];
}

// 这是最外层的标准 API 响应结构
export interface ApiResult<T> {
  code: number;
  message: string;
  data: T;
}

export const uploadService = {
  /**
   * Batch upload multiple files.
   * The Axios response `data` will be wrapped in our standard ApiResult.
   */
  batchUploadFiles: async (
    files: { file: File; hash: string }[],
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<BatchUploadData>>> => {
    const formData = new FormData();

    files.forEach((fileObj) => {
      formData.append(fileObj.hash, fileObj.file, fileObj.file.name);
    });

    // T in api.post<T> now refers to the entire { code, message, data } object
    return api.post<ApiResult<BatchUploadData>>(
      "/api/v1/assets/batch",
      formData,
      {
        ...config,
        headers: {
          "Content-Type": "multipart/form-data",
          ...(config?.headers ?? {}),
        },
      },
    );
  },

  /**
   * Upload a file to the server
   * @param file - The file to upload
   * @param hash - Unique file identifier
   * @param config - Optional Axios config (e.g., onUploadProgress)
   * @returns A promise resolving to an Axios response with UploadResponse
   */
  uploadFile: async (
    file: File,
    hash: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<UploadResponse>> => {
    const formData = new FormData();
    formData.append("file", file);

    // Add the hash as a header instead of form data
    return api.post<UploadResponse>("/api/v1/assets", formData, {
      ...config,
      headers: {
        "Content-Type": "multipart/form-data",
        "X-Content-Hash": hash,
        ...(config?.headers ?? {}),
      },
    });
  },
};

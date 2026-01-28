import { getToken } from "@/lib/http-commons/api";
import type {
  ApiResult,
  UploadResponse,
  BatchUploadResponse,
  UploadOptions,
  BatchUploadOptions,
  BatchUploadFile,
  ChunkedUploadOptions,
} from "@/lib/upload/types";

const baseURL = import.meta.env.VITE_API_URL || "http://localhost:8080";

const attachAuthHeader = (headers: Record<string, string>) => {
  const token = getToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
};

export const uploadFile = async (
  file: File,
  hash: string,
  options?: UploadOptions,
): Promise<ApiResult<UploadResponse>> => {
  const formData = new FormData();
  formData.append("file", file);

  const headers: Record<string, string> = {
    "X-Content-Hash": hash,
  };

  attachAuthHeader(headers);

  if (options?.onUploadProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${baseURL}/assets`);

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

  const response = await fetch(`${baseURL}/assets`, {
    method: "POST",
    headers,
    body: formData,
    signal: options?.signal,
  });

  return response.json();
};

export const batchUploadFiles = async (
  files: BatchUploadFile[],
  repositoryId?: string,
  options?: BatchUploadOptions,
): Promise<ApiResult<BatchUploadResponse>> => {
  const formData = new FormData();

  if (repositoryId) {
    formData.append("repository_id", repositoryId);
  }

  files.forEach((fileObj) => {
    let fieldName: string;

    if (
      fileObj.isChunk &&
      fileObj.chunkIndex !== undefined &&
      fileObj.totalChunks !== undefined
    ) {
      fieldName = `chunk_${fileObj.sessionId}_${fileObj.chunkIndex}_${fileObj.totalChunks}`;
    } else {
      fieldName = `single_${fileObj.sessionId}`;
    }

    const filename =
      fileObj.fileName || (fileObj.file as File).name || "upload";
    formData.append(fieldName, fileObj.file, filename);
  });

  const headers: Record<string, string> = {};

  if (options?.contentHash) {
    headers["X-Content-Hash"] = options.contentHash;
  }

  attachAuthHeader(headers);

  if (options?.onUploadProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${baseURL}/assets/batch`);

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

  const response = await fetch(`${baseURL}/assets/batch`, {
    method: "POST",
    headers,
    body: formData,
    signal: options?.signal,
  });

  return response.json();
};

export const uploadFileInChunks = async (
  file: File,
  sessionId: string,
  hash: string,
  chunkSize: number = 24 * 1024 * 1024,
  repositoryId?: string,
  onProgress?: (progress: number) => void,
  options?: ChunkedUploadOptions,
): Promise<ApiResult<BatchUploadResponse>> => {
  const effectiveChunkSize = options?.chunkSize ?? chunkSize;
  const totalChunks = Math.ceil(file.size / effectiveChunkSize);
  const maxConcurrent = options?.maxConcurrent ?? 6;
  let lastResponse: ApiResult<BatchUploadResponse> | null = null;

  const uploadChunk = async (chunkIndex: number) => {
    const start = chunkIndex * effectiveChunkSize;
    const end = Math.min(start + effectiveChunkSize, file.size);
    const chunk = file.slice(start, end);

    return batchUploadFiles(
      [
        {
          file: chunk,
          fileName: file.name,
          sessionId,
          isChunk: true,
          chunkIndex,
          totalChunks,
        },
      ],
      repositoryId,
      {
        contentHash: hash,
      },
    );
  };

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
          .then((response) => {
            lastResponse = response;
            completedChunks++;
            activeUploads--;

            if (onProgress) {
              const progress = Math.min(
                (completedChunks / totalChunks) * 100,
                100,
              );
              onProgress(progress);
            }

            processNext();
          })
          .catch((error) => {
            reject(error);
          });
      }
    };

    processNext();
  });
};

export const generateSessionId = (): string => {
  return crypto.randomUUID();
};

export const shouldUseChunks = (
  file: File,
  threshold: number = 10 * 1024 * 1024,
): boolean => {
  return file.size > threshold;
};

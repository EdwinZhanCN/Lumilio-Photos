import { getToken } from "@/lib/http-commons/auth.ts";
import type {
  UploadResponse,
  BatchUploadResponse,
  UploadOptions,
  BatchUploadOptions,
  BatchUploadFile,
  ChunkedUploadOptions,
  UploadPrecheckFile,
  UploadPrecheckResponse,
  UploadSessionState,
} from "@/lib/upload/types";

const baseURL = import.meta.env.VITE_API_URL ?? "";

const responseError = async (response: Response, operation: string): Promise<Error> => {
  let detail = "";
  try {
    const payload: unknown = await response.json();
    if (payload && typeof payload === "object") {
      if ("error" in payload && typeof payload.error === "string") detail = payload.error;
      else if ("message" in payload && typeof payload.message === "string") detail = payload.message;
    }
  } catch {
    // The status code remains actionable when the response has no JSON body.
  }
  return new Error(`${operation} failed with status ${response.status}${detail ? `: ${detail}` : ""}`);
};

const parseSuccessfulJSON = async <T>(response: Response, operation: string): Promise<T> => {
  if (!response.ok) throw await responseError(response, operation);
  return response.json() as Promise<T>;
};

const parseSuccessfulXHR = <T>(xhr: XMLHttpRequest, operation: string): T => {
  if (xhr.status < 200 || xhr.status >= 300) {
    throw new Error(`${operation} failed with status ${xhr.status}`);
  }
  try {
    return JSON.parse(xhr.responseText) as T;
  } catch {
    throw new Error(`${operation} returned an invalid response`);
  }
};

const attachAuthHeader = (headers: Record<string, string>) => {
  const token = getToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
};

/**
 * Asks the server which of the given fingerprints already exist in the target
 * repository. Files reported as duplicates never need their bytes transported.
 */
export const precheckUploads = async (
  files: UploadPrecheckFile[],
  repositoryId?: string,
  signal?: AbortSignal,
): Promise<UploadPrecheckResponse> => {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  attachAuthHeader(headers);

  const response = await fetch(`${baseURL}/api/v1/assets/precheck`, {
    method: "POST",
    headers,
    body: JSON.stringify({ files, repository_id: repositoryId }),
    signal,
  });

  return parseSuccessfulJSON<UploadPrecheckResponse>(response, "Upload precheck");
};

export const uploadFile = async (
  file: File,
  hash: string,
  options?: UploadOptions,
): Promise<UploadResponse> => {
  const formData = new FormData();
  formData.append("file", file);

  const headers: Record<string, string> = {
    "X-Upload-Fingerprint": hash,
  };

  attachAuthHeader(headers);

  if (options?.onUploadProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${baseURL}/api/v1/assets`);

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
          resolve(parseSuccessfulXHR<UploadResponse>(xhr, "Upload"));
        } catch (error) {
          reject(error);
        }
      };

      xhr.onerror = () => reject(new Error("Upload failed"));
      xhr.onabort = () => reject(new DOMException("Upload aborted", "AbortError"));

      if (options.signal) {
        if (options.signal.aborted) {
          reject(new DOMException("Upload aborted", "AbortError"));
          return;
        }
        options.signal.addEventListener("abort", () => xhr.abort());
      }

      xhr.send(formData);
    });
  }

  const response = await fetch(`${baseURL}/api/v1/assets`, {
    method: "POST",
    headers,
    body: formData,
    signal: options?.signal,
  });

  return parseSuccessfulJSON<UploadResponse>(response, "Upload");
};

export const batchUploadFiles = async (
  files: BatchUploadFile[],
  repositoryId?: string,
  options?: BatchUploadOptions,
): Promise<BatchUploadResponse> => {
  const formData = new FormData();

  if (repositoryId) {
    formData.append("repository_id", repositoryId);
  }

  files.forEach((fileObj) => {
    let fieldName: string;

    if (fileObj.isChunk && fileObj.chunkIndex !== undefined && fileObj.totalChunks !== undefined) {
      fieldName = `chunk_${fileObj.sessionId}_${fileObj.chunkIndex}_${fileObj.totalChunks}`;
    } else {
      fieldName = `single_${fileObj.sessionId}`;
    }

    const filename = fileObj.fileName || (fileObj.file as File).name || "upload";
    formData.append(fieldName, fileObj.file, filename);
  });

  const headers: Record<string, string> = {};

  if (options?.contentHash) {
    headers["X-Upload-Fingerprint"] = options.contentHash;
  }

  attachAuthHeader(headers);

  if (options?.onUploadProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${baseURL}/api/v1/assets/batch`);

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
          resolve(parseSuccessfulXHR<BatchUploadResponse>(xhr, "Batch upload"));
        } catch (error) {
          reject(error);
        }
      };

      xhr.onerror = () => reject(new Error("Batch upload failed"));
      xhr.onabort = () => reject(new DOMException("Batch upload aborted", "AbortError"));

      if (options.signal) {
        if (options.signal.aborted) {
          reject(new DOMException("Batch upload aborted", "AbortError"));
          return;
        }
        options.signal.addEventListener("abort", () => xhr.abort());
      }

      xhr.send(formData);
    });
  }

  const response = await fetch(`${baseURL}/api/v1/assets/batch`, {
    method: "POST",
    headers,
    body: formData,
    signal: options?.signal,
  });

  return parseSuccessfulJSON<BatchUploadResponse>(response, "Batch upload");
};

export const uploadFileInChunks = async (
  file: File,
  sessionId: string,
  hash: string,
  chunkSize: number = 24 * 1024 * 1024,
  repositoryId?: string,
  onProgress?: (progress: number) => void,
  options?: ChunkedUploadOptions,
): Promise<BatchUploadResponse> => {
  const effectiveChunkSize = options?.chunkSize ?? chunkSize;
  const totalChunks = Math.ceil(file.size / effectiveChunkSize);
  const maxConcurrent = options?.maxConcurrent ?? 6;
  let lastResponse: BatchUploadResponse | null = null;

  const session = await createUploadSession({
    session_id: sessionId,
    filename: file.name,
    total_size: file.size,
    total_chunks: totalChunks,
    content_type: file.type,
    repository_id: repositoryId,
    client_fingerprint: hash,
  });
  if (session.status === "completed" && session.task_id) {
    return { results: [{ success: true, file_name: file.name, content_hash: "", task_id: session.task_id, status: "processing" }] };
  }
  const completed = new Set(session.received_chunks ?? []);

  const uploadChunk = async (chunkIndex: number) => {
    const start = chunkIndex * effectiveChunkSize;
    const end = Math.min(start + effectiveChunkSize, file.size);
    const chunk = file.slice(start, end);

    let lastError: unknown;
    for (let attempt = 0; attempt < 4; attempt++) {
      try {
        return await batchUploadFiles(
          [{ file: chunk, fileName: file.name, sessionId, isChunk: true, chunkIndex, totalChunks }],
          repositoryId,
          { contentHash: hash },
        );
      } catch (error) {
        lastError = error;
        if (attempt < 3) await new Promise((resolve) => globalThis.setTimeout(resolve, 300 * 2 ** attempt));
      }
    }
    throw lastError;
  };

  let activeUploads = 0;
  const pendingChunks = Array.from({ length: totalChunks }, (_, index) => index).filter((index) => !completed.has(index));
  // If every chunk reached the server but the completion response was lost,
  // replay the final chunk as an idempotent finalize ping.
  if (pendingChunks.length === 0 && totalChunks > 0) pendingChunks.push(totalChunks - 1);
  let nextPendingIndex = 0;
  let completedChunks = completed.size;
  let stopped = false;

  return new Promise((resolve, reject) => {
    const processNext = () => {
      if (stopped) return;
      if (nextPendingIndex >= pendingChunks.length) {
        if (activeUploads === 0 && lastResponse) {
          resolve(lastResponse);
        }
        return;
      }

      while (activeUploads < maxConcurrent && nextPendingIndex < pendingChunks.length) {
		const currentIndex = pendingChunks[nextPendingIndex++];
        activeUploads++;

        uploadChunk(currentIndex)
          .then((response) => {
            lastResponse = response;
            completedChunks++;
            activeUploads--;

            if (onProgress) {
              const progress = Math.min((completedChunks / totalChunks) * 100, 100);
              onProgress(progress);
            }

            processNext();
          })
          .catch((error) => {
            stopped = true;
            reject(error);
          });
      }
    };

    processNext();
  });
};

export const createUploadSession = async (request: {
  session_id?: string; filename: string; total_size: number; total_chunks: number;
  content_type?: string; repository_id?: string; client_fingerprint?: string;
}): Promise<UploadSessionState> => {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  attachAuthHeader(headers);
  const response = await fetch(`${baseURL}/api/v1/assets/batch/sessions`, {
    method: "POST", headers, body: JSON.stringify(request),
  });
  return parseSuccessfulJSON<UploadSessionState>(response, "Upload session");
};

export const generateSessionId = (): string => {
  return crypto.randomUUID();
};

export const getResumableSessionId = (file: File, repositoryId?: string): string => {
  const key = resumableSessionKey(file, repositoryId);
  try {
    const existing = localStorage.getItem(key);
    if (existing) return existing;
    const created = generateSessionId();
    localStorage.setItem(key, created);
    return created;
  } catch {
    return generateSessionId();
  }
};

const resumableSessionKey = (file: File, repositoryId?: string): string =>
  `lumilio.upload.session.v1:${repositoryId ?? "primary"}:${file.name}:${file.size}:${file.lastModified}`;

export const clearResumableSessionId = (file: File, repositoryId?: string): void => {
  try { localStorage.removeItem(resumableSessionKey(file, repositoryId)); } catch { /* storage is optional */ }
};

export const shouldUseChunks = (file: File, threshold: number = 10 * 1024 * 1024): boolean => {
  return file.size > threshold;
};

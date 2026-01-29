import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type UploadResponse = Schemas["dto.UploadResponseDTO"];
export type BatchUploadResponse = Schemas["dto.BatchUploadResponseDTO"];
export type BatchUploadResult = Schemas["dto.BatchUploadResultDTO"];
export type UploadConfigResponse = Schemas["dto.UploadConfigResponseDTO"];
export type UploadProgressResponse = Schemas["dto.UploadProgressResponseDTO"];
export type SessionProgress = Schemas["dto.SessionProgressDTO"];

export type UploadProgressEvent = {
  loaded: number;
  total: number;
  percent: number;
};

export interface UploadOptions {
  onUploadProgress?: (progress: UploadProgressEvent) => void;
  signal?: AbortSignal;
}

export interface BatchUploadFile {
  file: Blob;
  fileName?: string;
  sessionId: string;
  isChunk?: boolean;
  chunkIndex?: number;
  totalChunks?: number;
}

export interface BatchUploadOptions extends UploadOptions {
  contentHash?: string;
}

export interface ChunkedUploadOptions {
  maxConcurrent?: number;
  chunkSize?: number;
}

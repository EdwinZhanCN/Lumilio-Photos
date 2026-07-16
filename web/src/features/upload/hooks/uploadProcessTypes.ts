import type { BatchUploadResult } from "@/lib/upload/types";

export interface FailedFile {
  name: string;
  error: string;
  file: File;
}

export interface ProcessResults {
  uploaded: string[];
  duplicates: string[];
  failed: FailedFile[];
}

export type ProcessFilesFn = (files: FileList | File[]) => Promise<ProcessResults>;

export type FileUploadStatus =
  | "pending"
  | "uploading"
  | "processing"
  | "completed"
  | "duplicate"
  | "failed";

export interface FileUploadProgress {
  fileName: string;
  progress: number;
  status: FileUploadStatus;
  sessionId: string;
  isChunked: boolean;
  error?: string;
}

export interface PlannedFileUploadSession {
  file: File;
  sessionId: string;
  shouldUseChunks: boolean;
}

export interface FileUploadSession extends PlannedFileUploadSession {
  hash: string;
}

export interface UploadTransportConfig {
  maxConcurrentUploads: number;
  chunkConcurrency: number;
  chunkSize: number;
}

export interface UploadProcessMessages {
  noResult: string;
  processFailed: string;
  uploadFailed: string;
}

export interface UploadProgressCallbacks {
  updateFileProgress: (sessionId: string, updates: Partial<FileUploadProgress>) => void;
  setUploadProgress: (progress: number) => void;
}

export interface UploadRunResult {
  results: BatchUploadResult[];
  resultSessions: Map<BatchUploadResult, FileUploadSession>;
}

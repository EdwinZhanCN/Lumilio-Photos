import type { BatchUploadResult } from "@/lib/upload/types";
import type { FileUploadSession, ProcessResults } from "./types.ts";
import type { FileUploadStatus } from "./types.ts";

// Server-side status marking content that already exists in the repository, so
// the upload was satisfied without transporting (or ingesting) the bytes.
export const DUPLICATE_STATUS = "duplicate";

export const isDuplicateResult = (result: BatchUploadResult): boolean =>
  result.status === DUPLICATE_STATUS;

export const resolveResultStatus = (result: BatchUploadResult): FileUploadStatus => {
  if (isDuplicateResult(result)) return "duplicate";
  if (!result.success || !result.task_id) return "failed";
  return "processing";
};

export const summarizeUploadResults = (
  results: BatchUploadResult[],
  resultSessions: Map<BatchUploadResult, FileUploadSession>,
  fallbackFile: File,
  uploadFailedMessage: string,
): ProcessResults => {
  const duplicates = results.filter(isDuplicateResult).map((result) => result.file_name || "");
  const uploaded = results
    .filter((result) => result.success && !isDuplicateResult(result))
    .map((result) => result.file_name || "");
  const failed = results
    .filter((result) => !result.success)
    .map((result) => ({
      name: result.file_name || "Unknown",
      error: result.message || result.error || uploadFailedMessage,
      file: resultSessions.get(result)?.file ?? fallbackFile,
    }));

  return { uploaded, duplicates, failed };
};

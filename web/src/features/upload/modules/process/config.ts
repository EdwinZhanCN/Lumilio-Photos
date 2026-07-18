import type { UploadConfigResponse } from "@/lib/upload/types";
import type { UploadTransportConfig } from "./types.ts";

// Transport fallbacks used only while the server upload config is unavailable.
// The server endpoint is the source of truth for these values.
export const FALLBACK_CHUNK_SIZE = 5 * 1024 * 1024;
export const FALLBACK_MAX_CONCURRENT = 3;
export const FALLBACK_MAX_IN_FLIGHT = 3;
export const QUICK_HASH_THRESHOLD = 100 * 1024 * 1024;
export const QUICK_FINGERPRINT_VERSION = "blake3-size-first-last-1m-v1";

export const toPositiveInt = (value: number | undefined, fallback: number): number => {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return Math.max(1, Math.floor(fallback));
  }
  return Math.max(1, Math.floor(value));
};

export const resolveUploadTransportConfig = (
  serverConfig: UploadConfigResponse | undefined,
): UploadTransportConfig => ({
  maxConcurrentUploads: toPositiveInt(serverConfig?.max_in_flight_requests, FALLBACK_MAX_IN_FLIGHT),
  chunkConcurrency: toPositiveInt(serverConfig?.max_concurrent, FALLBACK_MAX_CONCURRENT),
  chunkSize: toPositiveInt(serverConfig?.chunk_size, FALLBACK_CHUNK_SIZE),
});

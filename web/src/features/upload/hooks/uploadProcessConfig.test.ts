import { describe, expect, it } from "vite-plus/test";
import {
  FALLBACK_CHUNK_SIZE,
  FALLBACK_MAX_CONCURRENT,
  FALLBACK_MAX_IN_FLIGHT,
  resolveUploadTransportConfig,
} from "./uploadProcessConfig";

describe("resolveUploadTransportConfig", () => {
  it("uses resilient fallbacks when server config is unavailable", () => {
    expect(resolveUploadTransportConfig(undefined)).toEqual({
      maxConcurrentUploads: FALLBACK_MAX_IN_FLIGHT,
      chunkConcurrency: FALLBACK_MAX_CONCURRENT,
      chunkSize: FALLBACK_CHUNK_SIZE,
    });
  });

  it("normalizes invalid server values and keeps valid values", () => {
    expect(
      resolveUploadTransportConfig({
        max_in_flight_requests: 4.9,
        max_concurrent: 0,
        chunk_size: Number.NaN,
      }),
    ).toEqual({
      maxConcurrentUploads: 4,
      chunkConcurrency: FALLBACK_MAX_CONCURRENT,
      chunkSize: FALLBACK_CHUNK_SIZE,
    });
  });
});

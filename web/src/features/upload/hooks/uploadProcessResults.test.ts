import { describe, expect, it } from "vite-plus/test";
import type { BatchUploadResult } from "@/lib/upload/types";
import {
  isDuplicateResult,
  resolveResultStatus,
  summarizeUploadResults,
} from "./uploadProcessResults";
import type { FileUploadSession } from "./uploadProcessTypes";

describe("upload process results", () => {
  it("classifies duplicate, processing, and failed results", () => {
    expect(isDuplicateResult({ status: "duplicate" })).toBe(true);
    expect(resolveResultStatus({ status: "duplicate" })).toBe("duplicate");
    expect(resolveResultStatus({ success: true, task_id: 42 })).toBe("processing");
    expect(resolveResultStatus({ success: false })).toBe("failed");
  });

  it("keeps duplicate files out of the uploaded count", () => {
    const file = new File(["photo"], "photo.jpg");
    const session: FileUploadSession = {
      file,
      sessionId: "session-1",
      hash: "hash-1",
      shouldUseChunks: false,
    };
    const duplicate: BatchUploadResult = {
      success: true,
      file_name: file.name,
      status: "duplicate",
    };
    const failed: BatchUploadResult = {
      success: false,
      file_name: "broken.jpg",
      error: "network error",
    };

    const summary = summarizeUploadResults(
      [duplicate, failed],
      new Map([
        [duplicate, session],
        [failed, session],
      ]),
      file,
      "Upload failed",
    );

    expect(summary.uploaded).toEqual([]);
    expect(summary.duplicates).toEqual([file.name]);
    expect(summary.failed).toEqual([{ name: "broken.jpg", error: "network error", file }]);
  });
});

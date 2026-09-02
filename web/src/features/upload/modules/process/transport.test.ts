import { describe, expect, it } from "vite-plus/test";
import type { BatchUploadResult } from "@/lib/upload/types";
import { matchBatchUploadResult } from "./transport.ts";
import type { FileUploadSession } from "./types.ts";

const session = (name: string, sessionId: string, uploadSessionId?: string): FileUploadSession => ({
  file: new File(["x"], name),
  sessionId,
  uploadSessionId,
  hash: "hash",
  shouldUseChunks: false,
});

describe("matchBatchUploadResult", () => {
  it("matches by session_id even when file names collide", () => {
    const a = session("DSC_0001.JPG", "ui-a", "upload-a");
    const b = session("DSC_0001.JPG", "ui-b", "upload-b");
    const byId = new Map([
      ["upload-a", a],
      ["upload-b", b],
    ]);
    const byName = new Map<string, FileUploadSession[]>([["DSC_0001.JPG", [a, b]]]);

    const first: BatchUploadResult = {
      success: true,
      session_id: "upload-b",
      file_name: "DSC_0001.JPG",
      receipt_id: "receipt-b",
    };
    expect(matchBatchUploadResult(first, byId, byName)).toBe(b);
    expect(byId.has("upload-b")).toBe(false);

    const second: BatchUploadResult = {
      success: true,
      session_id: "upload-a",
      file_name: "DSC_0001.JPG",
      receipt_id: "receipt-a",
    };
    expect(matchBatchUploadResult(second, byId, byName)).toBe(a);
  });

  it("falls back to file_name when session_id is absent", () => {
    const only = session("photo.jpg", "ui-1");
    const byId = new Map([["ui-1", only]]);
    const byName = new Map<string, FileUploadSession[]>([["photo.jpg", [only]]]);
    const matched = matchBatchUploadResult(
      { success: true, file_name: "photo.jpg", receipt_id: "receipt" },
      byId,
      byName,
    );
    expect(matched).toBe(only);
    expect(byId.size).toBe(0);
  });
});

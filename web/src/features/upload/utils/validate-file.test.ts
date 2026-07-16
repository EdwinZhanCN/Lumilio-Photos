import { describe, expect, it } from "vite-plus/test";

import { validateFile } from "./validate-file.ts";

describe("validateFile", () => {
  it("returns the canonical MIME type from the filename extension", () => {
    const file = new File(["data"], "photo.JPG", { type: "video/mp4" });

    const result = validateFile(file);

    expect(result.valid).toBe(true);
    expect(result.assetType).toBe("photo");
    expect(result.mimeType).toBe("image/jpeg");
    expect(result.isRAW).toBe(false);
  });

  it("marks RAW formats from the extension", () => {
    const file = new File(["data"], "capture.cr3", { type: "" });

    const result = validateFile(file);

    expect(result.valid).toBe(true);
    expect(result.assetType).toBe("photo");
    expect(result.mimeType).toBe("image/x-canon-cr3");
    expect(result.isRAW).toBe(true);
  });

  it("rejects unsupported extensions", () => {
    const file = new File(["data"], "notes.txt", { type: "text/plain" });

    const result = validateFile(file);

    expect(result.valid).toBe(false);
    expect(result.errorReason).toBe("Unsupported file extension: .txt");
  });
});

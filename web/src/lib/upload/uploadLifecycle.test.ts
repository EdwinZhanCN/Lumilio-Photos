import { describe, expect, it } from "vite-plus/test";
import { chunkUploadReceiptIds, UPLOAD_RECEIPT_ID_LIMIT } from "./uploadLifecycle";

describe("chunkUploadReceiptIds", () => {
  it("returns empty for no ids", () => {
    expect(chunkUploadReceiptIds([])).toEqual([]);
  });

  it("dedupes and keeps a single batch under the limit", () => {
    expect(chunkUploadReceiptIds(["a", "a", "b", "c"])).toEqual([["a", "b", "c"]]);
  });

  it("splits at the backend limit", () => {
    const ids = Array.from(
      { length: UPLOAD_RECEIPT_ID_LIMIT + 5 },
      (_, index) => `receipt-${index + 1}`,
    );
    const chunks = chunkUploadReceiptIds(ids);
    expect(chunks).toHaveLength(2);
    expect(chunks[0]).toHaveLength(UPLOAD_RECEIPT_ID_LIMIT);
    expect(chunks[1]).toEqual([
      "receipt-101",
      "receipt-102",
      "receipt-103",
      "receipt-104",
      "receipt-105",
    ]);
  });
});

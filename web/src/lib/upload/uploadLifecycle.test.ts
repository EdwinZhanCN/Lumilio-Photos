import { describe, expect, it } from "vite-plus/test";
import { chunkUploadJobIds, UPLOAD_JOB_ID_LIMIT } from "./uploadLifecycle";

describe("chunkUploadJobIds", () => {
  it("returns empty for no ids", () => {
    expect(chunkUploadJobIds([])).toEqual([]);
  });

  it("dedupes and keeps a single batch under the limit", () => {
    expect(chunkUploadJobIds([1, 1, 2, 3])).toEqual([[1, 2, 3]]);
  });

  it("splits at the backend limit", () => {
    const ids = Array.from({ length: UPLOAD_JOB_ID_LIMIT + 5 }, (_, index) => index + 1);
    const chunks = chunkUploadJobIds(ids);
    expect(chunks).toHaveLength(2);
    expect(chunks[0]).toHaveLength(UPLOAD_JOB_ID_LIMIT);
    expect(chunks[1]).toEqual([101, 102, 103, 104, 105]);
  });
});

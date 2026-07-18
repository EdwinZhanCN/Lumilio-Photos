import { describe, expect, it, vi } from "vite-plus/test";
import { initialState, uploadReducer } from "./reducer";

describe("uploadReducer", () => {
  it("retains failed files and releases successful previews", () => {
    const succeeded = new File(["ok"], "ok.jpg");
    const failed = new File(["retry"], "retry.jpg");
    const revoke = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);

    const next = uploadReducer(
      { ...initialState, files: [succeeded, failed], previews: ["blob:ok", "blob:retry"] },
      { type: "RETAIN_FILES", payload: [failed] },
    );

    expect(next.files).toEqual([failed]);
    expect(next.previews).toEqual(["blob:retry"]);
    expect(revoke).toHaveBeenCalledWith("blob:ok");
  });
});

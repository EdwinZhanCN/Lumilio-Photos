import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { precheckUploads, uploadFile } from "./uploadTransport";

describe("upload transport error mapping", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("rejects with the status and server message on a 503 upload response", async () => {
    vi.stubGlobal(
      "fetch",
      async () =>
        new Response(JSON.stringify({ message: "storage unavailable" }), {
          status: 503,
          headers: { "content-type": "application/json" },
        }),
    );

    await expect(uploadFile(new File(["photo"], "photo.jpg"), "smoke-hash")).rejects.toThrow(
      "Upload failed with status 503: storage unavailable",
    );
  });

  it("keeps the status actionable when the error body is not JSON", async () => {
    vi.stubGlobal(
      "fetch",
      async () =>
        new Response("<html>bad gateway</html>", {
          status: 503,
          headers: { "content-type": "text/html" },
        }),
    );

    await expect(precheckUploads([{ hash: "abcd", size: 5 }])).rejects.toThrow(
      "Upload precheck failed with status 503",
    );
  });
});

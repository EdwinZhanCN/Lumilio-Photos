import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import {
  clearResumableSessionId,
  getResumableSessionId,
  precheckUploads,
  uploadFile,
} from "./uploadTransport";

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

describe("resumable session keys", () => {
  const store = new Map<string, string>();

  beforeEach(() => {
    store.clear();
    vi.stubGlobal("localStorage", {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
      clear: () => {
        store.clear();
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("scopes resumable ids by content hash", () => {
    const file = new File(["photo"], "photo.jpg");
    const first = getResumableSessionId(file, "repo", "hash-a");
    const same = getResumableSessionId(file, "repo", "hash-a");
    const other = getResumableSessionId(file, "repo", "hash-b");
    expect(same).toBe(first);
    expect(other).not.toBe(first);
  });

  it("migrates a legacy name-size-mtime key into the hash-scoped key", () => {
    const file = new File(["photo"], "photo.jpg");
    Object.defineProperty(file, "lastModified", { value: 123 });
    const legacyKey = `lumilio.upload.session.v1:repo:${file.name}:${file.size}:${file.lastModified}`;
    localStorage.setItem(legacyKey, "legacy-session");
    expect(getResumableSessionId(file, "repo", "hash-a")).toBe("legacy-session");
    expect(localStorage.getItem(legacyKey)).toBeNull();
    clearResumableSessionId(file, "repo", "hash-a");
    expect(getResumableSessionId(file, "repo", "hash-a")).not.toBe("legacy-session");
  });
});

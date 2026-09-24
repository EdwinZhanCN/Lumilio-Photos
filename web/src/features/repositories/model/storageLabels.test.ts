import { describe, expect, it } from "vite-plus/test";
import { deriveStorageLabel } from "./storageLabels";

describe("deriveStorageLabel", () => {
  it("reports the machine's own storage as one system label", () => {
    for (const mountPath of [
      "/",
      "/System/Volumes/Data",
      "/System/Volumes/Preboot",
      "C:\\",
      "C:",
    ]) {
      expect(deriveStorageLabel(mountPath), mountPath).toEqual({
        kind: "system",
        mountPath,
      });
    }
  });

  it("names an external mount by its last path segment", () => {
    expect(deriveStorageLabel("/Volumes/Backup")).toEqual({
      kind: "named",
      mountPath: "/Volumes/Backup",
      name: "Backup",
    });
    expect(deriveStorageLabel("/volume1")).toEqual({
      kind: "named",
      mountPath: "/volume1",
      name: "volume1",
    });
    expect(deriveStorageLabel("/mnt/media/")).toEqual({
      kind: "named",
      mountPath: "/mnt/media/",
      name: "media",
    });
    expect(deriveStorageLabel("D:\\")).toEqual({
      kind: "named",
      mountPath: "D:\\",
      name: "D:",
    });
  });

  it("keeps an unresolved mount explicit instead of inventing a name", () => {
    expect(deriveStorageLabel(undefined)).toEqual({ kind: "unknown" });
    expect(deriveStorageLabel(null)).toEqual({ kind: "unknown" });
    expect(deriveStorageLabel("   ")).toEqual({ kind: "unknown" });
  });
});

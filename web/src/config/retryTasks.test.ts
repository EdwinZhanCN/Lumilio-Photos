import { describe, expect, it } from "vite-plus/test";
import {
  getRetryTasksByCategoryForAssetType,
  isRetryTaskSupportedForAssetType,
} from "./retryTasks";

describe("retry task asset type support", () => {
  it("allows photo retry tasks for metadata, thumbnails, and ML", () => {
    expect(isRetryTaskSupportedForAssetType("analyze", "PHOTO")).toBe(true);
    expect(isRetryTaskSupportedForAssetType("derivatives", "PHOTO")).toBe(true);
    expect(isRetryTaskSupportedForAssetType("enrich", "PHOTO")).toBe(true);
    expect(isRetryTaskSupportedForAssetType("transcode", "PHOTO")).toBe(false);
  });

  it("allows video retry tasks for metadata, thumbnails, transcode, and enrichment", () => {
    const tasks = getRetryTasksByCategoryForAssetType("VIDEO");

    expect(tasks.metadata.map((task) => task.key)).toEqual(["analyze"]);
    expect(tasks.media.map((task) => task.key)).toEqual(["derivatives", "transcode"]);
    expect(tasks.ml.map((task) => task.key)).toEqual(["enrich"]);
  });

  it("allows audio retry tasks for metadata and transcode only", () => {
    const tasks = getRetryTasksByCategoryForAssetType("AUDIO");

    expect(tasks.metadata.map((task) => task.key)).toEqual(["analyze"]);
    expect(tasks.media.map((task) => task.key)).toEqual(["transcode"]);
    expect(tasks.ml).toEqual([]);
  });
});

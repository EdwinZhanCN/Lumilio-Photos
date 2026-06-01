import { describe, expect, it } from "vite-plus/test";
import {
  getRetryTasksByCategoryForAssetType,
  isRetryTaskSupportedForAssetType,
} from "./retryTasks";

describe("retry task asset type support", () => {
  it("allows photo retry tasks for metadata, thumbnails, and ML", () => {
    expect(isRetryTaskSupportedForAssetType("metadata_asset", "PHOTO")).toBe(
      true,
    );
    expect(isRetryTaskSupportedForAssetType("thumbnail_asset", "PHOTO")).toBe(
      true,
    );
    expect(isRetryTaskSupportedForAssetType("process_clip", "PHOTO")).toBe(
      true,
    );
    expect(isRetryTaskSupportedForAssetType("transcode_asset", "PHOTO")).toBe(
      false,
    );
  });

  it("allows video retry tasks for metadata, thumbnails, and transcode only", () => {
    const tasks = getRetryTasksByCategoryForAssetType("VIDEO");

    expect(tasks.metadata.map((task) => task.key)).toEqual(["metadata_asset"]);
    expect(tasks.media.map((task) => task.key)).toEqual([
      "thumbnail_asset",
      "transcode_asset",
    ]);
    expect(tasks.ml).toEqual([]);
  });

  it("allows audio retry tasks for metadata and transcode only", () => {
    const tasks = getRetryTasksByCategoryForAssetType("AUDIO");

    expect(tasks.metadata.map((task) => task.key)).toEqual(["metadata_asset"]);
    expect(tasks.media.map((task) => task.key)).toEqual(["transcode_asset"]);
    expect(tasks.ml).toEqual([]);
  });
});

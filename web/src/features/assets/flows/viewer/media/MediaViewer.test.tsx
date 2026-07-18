import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import MediaViewer from "./MediaViewer";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

vi.mock("@/lib/assets/assetUrls", () => ({
  assetUrls: {
    getThumbnailUrl: (assetId: string) => `/thumbnail/${assetId}`,
    getWebVideoUrl: (assetId: string) => `/video/${assetId}`,
  },
}));

vi.mock("../../../api/useAssetMediaItem", () => ({
  useAssetMediaItem: () => ({
    data: {
      media_item: {
        media_kind: "photo",
        primary_asset_id: "raw",
        components: [
          { asset_id: "raw", relation: "raw_original" },
          { asset_id: "jpeg", relation: "jpeg_original" },
        ],
      },
    },
  }),
}));

vi.mock("../useLivePhotoPlayback", () => ({
  useLivePhotoPlayback: () => ({
    videoRef: { current: null },
    isPlaying: false,
    handlePlay: vi.fn(),
    handleStop: vi.fn(),
    handleEnded: vi.fn(),
  }),
}));

afterEach(() => cleanup());

describe("MediaViewer RAW/JPEG component selection", () => {
  it("uses the controlled component for the image and reports tab changes", () => {
    const onSelectedAssetChange = vi.fn();
    const asset = {
      asset_id: "raw",
      original_filename: "photo.raw",
      type: "PHOTO",
    } as Asset;

    const { container } = render(
      <MediaViewer
        asset={asset}
        selectedAssetId="jpeg"
        onSelectedAssetChange={onSelectedAssetChange}
      />,
    );

    expect(screen.getByRole("img")).toHaveAttribute("src", "/thumbnail/jpeg");
    fireEvent.click(screen.getByRole("radio", { name: "RAW" }));
    expect(onSelectedAssetChange).toHaveBeenCalledWith("raw");
    expect(container.querySelector('[role="tablist"]')).toHaveClass("tabs", "tabs-box");
  });
});

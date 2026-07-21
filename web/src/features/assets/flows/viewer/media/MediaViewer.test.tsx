import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset, MediaItemByAssetResponse } from "@/lib/assets/types";
import MediaViewer from "./MediaViewer";

// Real component + real useAssetMediaItem query + real assetUrls; only the
// media-item HTTP response is mocked. The RAW/JPEG picker is the subject.
function serveMediaItem(assetId: string) {
  const response = {
    asset_id: assetId,
    media_item: {
      media_kind: "photo",
      primary_asset_id: "raw",
      components: [
        { asset_id: "raw", relation: "raw_original" },
        { asset_id: "jpeg", relation: "jpeg_original" },
      ],
    },
  } satisfies MediaItemByAssetResponse;

  worker.use(
    http.get("*/api/v1/assets/:id/media-item", () => HttpResponse.json(response)),
    // The <img> actually requests its thumbnail; the URL is the subject, not the
    // bytes, so answer with an empty 200 to keep the /api/ guard quiet.
    http.get("*/api/v1/assets/:id/thumbnail", () => new HttpResponse(null, { status: 200 })),
  );
}

describe("MediaViewer RAW/JPEG component selection", () => {
  it("uses the controlled component for the image and reports tab changes", async () => {
    serveMediaItem("raw");
    const onSelectedAssetChange = vi.fn();
    const asset = {
      asset_id: "raw",
      original_filename: "photo.raw",
      type: "PHOTO",
    } as Asset;

    const screen = await renderWithProviders(
      <MediaViewer
        asset={asset}
        selectedAssetId="jpeg"
        onSelectedAssetChange={onSelectedAssetChange}
      />,
      { router: false },
    );

    // The controlled selection ("jpeg") drives the real thumbnail URL.
    await expect.element(screen.getByRole("img")).toBeVisible();
    const img = screen.getByRole("img").element() as HTMLImageElement;
    expect(img.src).toContain("/api/v1/assets/jpeg/thumbnail");

    await screen.getByRole("radio", { name: t("assets.mediaViewer.componentRaw") }).click();
    expect(onSelectedAssetChange).toHaveBeenCalledWith("raw");
    await expect.element(screen.getByRole("tablist")).toHaveClass("tabs-box");
  });
});

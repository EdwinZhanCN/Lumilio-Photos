import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset, MediaItemByAssetResponse } from "@/lib/assets/types";
import MediaViewer from "./MediaViewer";

// Vidstack is a heavy playback boundary. This component test only needs to
// prove the viewer passes URL state to that boundary; real media playback stays
// in the Playwright E2E layer.
vi.mock("@vidstack/react", () => ({
  MediaPlayer: ({
    children,
    currentTime,
    title,
  }: {
    children?: ReactNode;
    currentTime?: number;
    title?: string;
  }) => (
    <div role="region" aria-label={title} data-current-time={currentTime}>
      {children}
    </div>
  ),
  MediaProvider: () => null,
}));

vi.mock("@vidstack/react/player/layouts/default", () => ({
  defaultLayoutIcons: {},
  DefaultVideoLayout: () => null,
}));

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

describe("MediaViewer video start time", () => {
  it("seeks to the semantic match timestamp from the URL", async () => {
    const asset = {
      asset_id: "video",
      original_filename: "semantic-match.mp4",
      mime_type: "video/mp4",
      type: "VIDEO",
    } as Asset;

    const screen = await renderWithProviders(<MediaViewer asset={asset} isActive={false} />, {
      route: "/assets/video?t_ms=12345",
    });

    await expect
      .element(screen.getByRole("region", { name: "semantic-match.mp4" }))
      .toHaveAttribute("data-current-time", "12.345");
  });
});

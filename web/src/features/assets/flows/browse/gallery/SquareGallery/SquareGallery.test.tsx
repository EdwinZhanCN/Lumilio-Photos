import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import type { Asset } from "@/lib/assets/types";
import { createBrowseGroupsFromAssets } from "../../../../model/browseItems";
import { AssetBrowserScope } from "../../selection/AssetBrowserScope";
import SquareGallery from "./SquareGallery";

// Real browser layout, matchMedia, ResizeObserver and IntersectionObserver drive
// the viewport windowing under test — the whole point of running in Chromium
// rather than approximating them. Only the leaf thumbnails are stubbed so the
// windowing is measured without loading thousands of images.
vi.mock("../media/MediaThumbnail", () => ({
  default: ({ asset }: { asset: Asset }) => <div>{asset.asset_id}</div>,
}));
vi.mock("../media/StackedThumbnail", () => ({
  default: ({ asset }: { asset: Asset }) => <div>{asset.asset_id}</div>,
}));

describe("SquareGallery large-library window", () => {
  it("keeps mounted media bounded for a 10,000-item fixture", async () => {
    const assets = Array.from(
      { length: 10_000 },
      (_, index): Asset => ({
        asset_id: `asset-${index}`,
        original_filename: `asset-${index}.jpg`,
      }),
    );

    await renderWithProviders(
      <AssetBrowserScope scopeId="square-gallery-test">
        <SquareGallery
          browseGroups={createBrowseGroupsFromAssets(assets)}
          openCarousel={vi.fn()}
          onLoadMore={vi.fn()}
          hasMore={false}
          isLoadingMore={false}
          columns={4}
        />
      </AssetBrowserScope>,
    );

    await vi.waitFor(() => {
      expect(document.querySelector("[data-gallery-total='10000']")).not.toBeNull();
    });
    expect(document.querySelectorAll("[data-asset-id]").length).toBeLessThanOrEqual(48);
  });

  it("uses layout width when an ancestor is transformed", async () => {
    const assets = Array.from(
      { length: 10 },
      (_, index): Asset => ({
        asset_id: `scaled-asset-${index}`,
        original_filename: `scaled-asset-${index}.jpg`,
      }),
    );

    await renderWithProviders(
      <div style={{ width: 640, transform: "scale(0.8)", transformOrigin: "top left" }}>
        <AssetBrowserScope
          scopeId="square-gallery-transform-test"
          initialSelection={{ enabled: true, selectionMode: "multiple" }}
        >
          <SquareGallery
            browseGroups={createBrowseGroupsFromAssets(assets)}
            openCarousel={vi.fn()}
            onLoadMore={vi.fn()}
            hasMore={false}
            isLoadingMore={false}
            columns={5}
          />
        </AssetBrowserScope>
      </div>,
    );

    await vi.waitFor(() => {
      const grid = document.querySelector("[data-gallery-total='10']") as HTMLElement | null;
      const item = document.querySelector("[data-asset-id='scaled-asset-0']");
      const cell = item?.parentElement;
      const columnCount = window.matchMedia("(min-width: 768px)").matches ? 5 : 2;
      const expectedCellWidth = ((grid?.clientWidth ?? 0) - 2 * (columnCount - 1)) / columnCount;

      expect(grid).not.toBeNull();
      expect(cell).not.toBeNull();
      expect(Math.abs((cell?.offsetWidth ?? 0) - expectedCellWidth)).toBeLessThan(1.5);
    });
  });
});

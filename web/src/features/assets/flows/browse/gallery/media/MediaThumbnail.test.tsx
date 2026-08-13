import { describe, expect, it } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import type { Asset } from "@/lib/assets/types";
import MediaThumbnail from "./MediaThumbnail";

describe("MediaThumbnail selection overlay", () => {
  it("does not create per-tile backdrop-filter layers", async () => {
    const asset: Asset = {
      asset_id: "selection-overlay-test",
      original_filename: "selection-overlay-test.jpg",
      type: "PHOTO",
    };

    await renderWithProviders(
      <div style={{ width: 200, height: 200 }}>
        <MediaThumbnail asset={asset} isSelectionMode />
      </div>,
    );

    const tile = document.querySelector("[role='button']");
    expect(tile).not.toBeNull();
    if (!tile) return;

    const backdropFilteredDescendants = Array.from(tile.querySelectorAll("*")).filter(
      (element) => getComputedStyle(element).backdropFilter !== "none",
    );
    expect(backdropFilteredDescendants).toHaveLength(0);
  });
});

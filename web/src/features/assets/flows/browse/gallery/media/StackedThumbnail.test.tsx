import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset } from "@/lib/assets/types";
import type { BrowseStackItem } from "../../../../types";
import StackedThumbnail from "./StackedThumbnail";

// The two heavy children are the boundaries of this component test: MediaThumbnail
// renders the real image tile, StackCarouselOverlay is exercised in its own spec.
// Stubs let us assert this component's own logic — stopPropagation and the
// focus-asset handoff — without dragging their internals in.
vi.mock("./MediaThumbnail", () => ({
  default: ({ onClick }: { onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void }) => (
    <button type="button" onClick={onClick}>
      thumbnail
    </button>
  ),
}));

const overlayProbe: { current: { open: boolean; focusAssetId?: string } | null } = {
  current: null,
};

vi.mock("./StackCarouselOverlay", () => ({
  default: ({ open, focusAssetId }: { open: boolean; focusAssetId?: string }) => {
    overlayProbe.current = { open, focusAssetId };
    return open ? <div>stack-carousel-overlay</div> : null;
  },
}));

const asset = {
  asset_id: "stack-cover",
  original_filename: "stack-cover.jpg",
  stack: {
    stack_id: "stack-1",
    stack_kind: "burst",
    stack_size: 3,
    stack_cover: true,
  },
} as Asset;

const plainBrowseStack: BrowseStackItem = {
  type: "stack",
  id: "stack:stack-1",
  stackId: "stack-1",
  representative: asset,
  assets: [asset],
  memberAssetIds: ["stack-cover", "stack-member"],
  matchedMemberIds: [],
};

describe("StackedThumbnail", () => {
  beforeEach(() => {
    overlayProbe.current = null;
  });

  it("opens the stack carousel overlay without triggering the tile click", async () => {
    const handleClick = vi.fn();

    const screen = await renderWithProviders(
      <StackedThumbnail
        asset={asset}
        stackInfo={asset.stack!}
        browseStack={plainBrowseStack}
        onClick={handleClick}
      />,
      { router: false },
    );

    await screen
      .getByRole("button", { name: t("assets.stackDetail.openButton", { count: 3 }) })
      .click();

    expect(handleClick).not.toHaveBeenCalled();
    await expect.element(screen.getByText("stack-carousel-overlay")).toBeVisible();
    expect(overlayProbe.current?.open).toBe(true);
    expect(overlayProbe.current?.focusAssetId).toBe("stack-cover");
  });

  it("keeps thumbnail click behavior unchanged", async () => {
    const handleClick = vi.fn();

    const screen = await renderWithProviders(
      <StackedThumbnail
        asset={asset}
        stackInfo={asset.stack!}
        browseStack={plainBrowseStack}
        onClick={handleClick}
      />,
      { router: false },
    );

    await screen.getByRole("button", { name: "thumbnail" }).click();

    expect(handleClick).toHaveBeenCalledTimes(1);
  });
});

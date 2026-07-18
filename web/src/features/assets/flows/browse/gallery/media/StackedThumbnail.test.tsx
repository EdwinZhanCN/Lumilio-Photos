import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import StackedThumbnail from "./StackedThumbnail";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

vi.mock("./MediaThumbnail", () => ({
  default: ({ onClick }: { onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void }) => (
    <button type="button" onClick={onClick}>
      thumbnail
    </button>
  ),
}));

let lastOverlayProps: {
  open: boolean;
  focusAssetId?: string;
} | null = null;

vi.mock("./StackCarouselOverlay", () => ({
  default: ({ open, focusAssetId }: { open: boolean; focusAssetId?: string }) => {
    lastOverlayProps = { open, focusAssetId };
    return open ? <div>stack-carousel-overlay</div> : null;
  },
}));

afterEach(() => {
  cleanup();
});

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

const plainBrowseStack = {
  type: "stack",
  id: "stack:stack-1",
  stackId: "stack-1",
  representative: asset,
  assets: [asset],
  memberAssetIds: ["stack-cover", "stack-member"],
  matchedMemberIds: [],
} as const;

describe("StackedThumbnail", () => {
  beforeEach(() => {
    lastOverlayProps = null;
  });

  it("opens the stack carousel overlay without triggering tile click", () => {
    const handleClick = vi.fn();

    render(
      <StackedThumbnail
        asset={asset}
        stackInfo={asset.stack!}
        browseStack={plainBrowseStack as any}
        onClick={handleClick}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /view 3 related assets/i }));

    expect(handleClick).not.toHaveBeenCalled();
    expect(screen.getByText("stack-carousel-overlay")).toBeInTheDocument();
    expect(lastOverlayProps?.open).toBe(true);
    expect(lastOverlayProps?.focusAssetId).toBe("stack-cover");
  });

  it("keeps thumbnail click behavior unchanged", () => {
    const handleClick = vi.fn();

    render(
      <StackedThumbnail
        asset={asset}
        stackInfo={asset.stack!}
        browseStack={plainBrowseStack as any}
        onClick={handleClick}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "thumbnail" }));

    expect(handleClick).toHaveBeenCalledTimes(1);
  });
});

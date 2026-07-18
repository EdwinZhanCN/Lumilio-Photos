import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import StackCarouselOverlay from "./StackCarouselOverlay";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

vi.mock("../../hooks/useStackCarouselAssets", () => ({
  useStackCarouselAssets: () => ({
    assets: [
      {
        asset_id: "cover",
        original_filename: "cover.jpg",
      },
      {
        asset_id: "member",
        original_filename: "member.jpg",
      },
    ] as Asset[],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("../page/FullScreen/FullScreenCarousel/FullScreenCarousel", () => ({
  default: ({ initialSlide, slideIndex }: { initialSlide: number; slideIndex?: number }) => (
    <div
      data-testid="fullscreen-carousel"
      data-initial-slide={initialSlide}
      data-slide-index={slideIndex ?? ""}
    />
  ),
}));

afterEach(() => {
  cleanup();
});

const asset = {
  asset_id: "cover",
  original_filename: "cover.jpg",
  stack: {
    stack_id: "stack-1",
    stack_size: 2,
    stack_cover: true,
  },
} as Asset;

describe("StackCarouselOverlay", () => {
  it("opens on the matched member when a focus asset is provided", () => {
    render(<StackCarouselOverlay asset={asset} focusAssetId="member" open onClose={vi.fn()} />);

    const carousel = screen.getByTestId("fullscreen-carousel");
    expect(carousel).toHaveAttribute("data-initial-slide", "1");
    expect(carousel).toHaveAttribute("data-slide-index", "1");
    expect(carousel.parentElement).toBe(document.body);
  });

  it("falls back to the cover when no matched member focus is provided", () => {
    render(<StackCarouselOverlay asset={asset} open onClose={vi.fn()} />);

    const carousel = screen.getByTestId("fullscreen-carousel");
    expect(carousel).toHaveAttribute("data-initial-slide", "0");
    expect(carousel).toHaveAttribute("data-slide-index", "0");
  });
});

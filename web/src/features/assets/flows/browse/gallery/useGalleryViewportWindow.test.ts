import { describe, expect, it } from "vite-plus/test";
import { intersectsGalleryWindow } from "./useGalleryViewportWindow";

describe("intersectsGalleryWindow", () => {
  const viewportWindow = { start: 1000, end: 2000 };

  it("keeps tiles intersecting either edge", () => {
    expect(intersectsGalleryWindow(900, 100, viewportWindow)).toBe(true);
    expect(intersectsGalleryWindow(2000, 100, viewportWindow)).toBe(true);
    expect(intersectsGalleryWindow(1500, 100, viewportWindow)).toBe(true);
  });

  it("drops tiles outside the retained window", () => {
    expect(intersectsGalleryWindow(0, 999, viewportWindow)).toBe(false);
    expect(intersectsGalleryWindow(2100, 100, viewportWindow)).toBe(false);
  });
});

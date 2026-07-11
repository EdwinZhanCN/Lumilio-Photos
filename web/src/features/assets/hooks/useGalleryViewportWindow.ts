import { useCallback, useEffect, useState, type RefObject } from "react";
import { getScrollParent } from "./useGalleryInfiniteScroll";

export const GALLERY_OVERSCAN_PX = 1200;

export type GalleryViewportWindow = {
  start: number;
  end: number;
};

const initialViewportWindow = (): GalleryViewportWindow => ({
  start: 0,
  end: (typeof window === "undefined" ? 800 : window.innerHeight) + GALLERY_OVERSCAN_PX,
});

/**
 * Returns the vertical slice of a gallery container that may mount media.
 * The container keeps its full measured height, while rows/tiles outside this
 * overscanned window are removed from the DOM and release their image/video
 * resources.
 */
export function useGalleryViewportWindow(
  containerRef: RefObject<HTMLElement | null>,
  contentHeight: number,
  overscan = GALLERY_OVERSCAN_PX,
): GalleryViewportWindow {
  const [viewportWindow, setViewportWindow] = useState(initialViewportWindow);

  const measure = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;

    const containerRect = container.getBoundingClientRect();
    const scrollParent = getScrollParent(container);
    const rootRect = scrollParent?.getBoundingClientRect();
    const viewportTop = rootRect?.top ?? 0;
    const viewportBottom = rootRect?.bottom ?? window.innerHeight;

    const next = {
      start: Math.max(0, viewportTop - containerRect.top - overscan),
      end: Math.min(contentHeight, viewportBottom - containerRect.top + overscan),
    };

    setViewportWindow((current) =>
      Math.abs(current.start - next.start) < 1 && Math.abs(current.end - next.end) < 1
        ? current
        : next,
    );
  }, [containerRef, contentHeight, overscan]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const scrollParent = getScrollParent(container);
    let frame = 0;
    const scheduleMeasure = () => {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        measure();
      });
    };

    measure();
    const eventTarget: HTMLElement | Window = scrollParent ?? window;
    eventTarget.addEventListener("scroll", scheduleMeasure, { passive: true });
    window.addEventListener("resize", scheduleMeasure, { passive: true });
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(scheduleMeasure);
    observer?.observe(container);
    if (scrollParent) observer?.observe(scrollParent);

    return () => {
      eventTarget.removeEventListener("scroll", scheduleMeasure);
      window.removeEventListener("resize", scheduleMeasure);
      observer?.disconnect();
      if (frame) cancelAnimationFrame(frame);
    };
  }, [containerRef, contentHeight, measure]);

  return viewportWindow;
}

export function intersectsGalleryWindow(
  top: number,
  height: number,
  viewportWindow: GalleryViewportWindow,
): boolean {
  return top + height >= viewportWindow.start && top <= viewportWindow.end;
}

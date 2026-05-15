import { type RefObject, useCallback, useEffect, useMemo, useRef } from "react";

export function getScrollParent(
  element: HTMLElement | null,
): HTMLElement | null {
  if (!element || typeof window === "undefined") return null;
  let current = element.parentElement;
  while (current) {
    const style = window.getComputedStyle(current);
    if (/(auto|scroll|overlay)/.test(style.overflowY)) {
      return current;
    }
    current = current.parentElement;
  }
  return null;
}

/** Keep in sync with IntersectionObserver rootMargin below */
const ROOT_MARGIN = "1200px 0px";

function sentinelShouldPrefetch(
  sentinel: HTMLElement,
  root: HTMLElement | null,
): boolean {
  const rect = sentinel.getBoundingClientRect();
  const margin = 1200;
  if (root) {
    const r = root.getBoundingClientRect();
    const expandedTop = r.top - margin;
    const expandedBottom = r.bottom + margin;
    return rect.top < expandedBottom && rect.bottom > expandedTop;
  }
  const vh = typeof window !== "undefined" ? window.innerHeight : 0;
  return rect.top < vh + margin && rect.bottom > -margin;
}

type Args = {
  sentinelRef: RefObject<HTMLDivElement | null>;
  hasMore: boolean;
  isLoadingMore: boolean;
  isLoading: boolean;
  onLoadMore: () => void;
  /** When the list length changes, re-bind the observer (sentinel stays at list end) */
  totalAssetCount: number;
};

/**
 * IntersectionObserver-based infinite scroll with a post-fetch catch-up:
 * when a page finishes loading while the sentinel stayed in view, IO often
 * does not emit again — we manually check geometry in that case.
 */
export function useGalleryInfiniteScroll({
  sentinelRef,
  hasMore,
  isLoadingMore,
  isLoading,
  onLoadMore,
  totalAssetCount,
}: Args): { supportsIntersectionObserver: boolean } {
  const onLoadMoreRef = useRef(onLoadMore);
  const hasMoreRef = useRef(hasMore);
  const isLoadingMoreRef = useRef(isLoadingMore);
  const isLoadingRef = useRef(isLoading);

  useEffect(() => {
    onLoadMoreRef.current = onLoadMore;
  }, [onLoadMore]);
  useEffect(() => {
    hasMoreRef.current = hasMore;
  }, [hasMore]);
  useEffect(() => {
    isLoadingMoreRef.current = isLoadingMore;
  }, [isLoadingMore]);
  useEffect(() => {
    isLoadingRef.current = isLoading;
  }, [isLoading]);

  const supportsIntersectionObserver = useMemo(() => {
    if (typeof window === "undefined") return false;
    return "IntersectionObserver" in window;
  }, []);

  const tryLoadMore = useCallback(() => {
    if (!hasMoreRef.current) return;
    if (isLoadingRef.current || isLoadingMoreRef.current) return;
    onLoadMoreRef.current();
  }, []);

  const wasLoadingMoreRef = useRef(false);
  useEffect(() => {
    const prev = wasLoadingMoreRef.current;
    wasLoadingMoreRef.current = isLoadingMore;
    if (!prev || isLoadingMore || !hasMore) return;

    let cancelled = false;
    const outer = requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (cancelled) return;
        const sentinel = sentinelRef.current;
        if (!sentinel || !hasMoreRef.current) return;
        if (isLoadingRef.current || isLoadingMoreRef.current) return;
        const root = getScrollParent(sentinel);
        if (sentinelShouldPrefetch(sentinel, root)) {
          tryLoadMore();
        }
      });
    });
    return () => {
      cancelled = true;
      cancelAnimationFrame(outer);
    };
  }, [hasMore, isLoadingMore, sentinelRef, tryLoadMore]);

  useEffect(() => {
    if (!supportsIntersectionObserver || !hasMore) return;
    const sentinel = sentinelRef.current;
    if (!sentinel) return;

    const root = getScrollParent(sentinel);
    let lastFired = 0;
    const THROTTLE_MS = 250;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (!entry?.isIntersecting) return;
        const now = Date.now();
        if (now - lastFired < THROTTLE_MS) return;
        if (!hasMoreRef.current) return;
        if (isLoadingRef.current || isLoadingMoreRef.current) return;
        lastFired = now;
        onLoadMoreRef.current();
      },
      {
        root,
        rootMargin: ROOT_MARGIN,
        threshold: 0,
      },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [supportsIntersectionObserver, hasMore, sentinelRef, totalAssetCount]);

  return { supportsIntersectionObserver };
}

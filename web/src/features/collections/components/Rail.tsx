import type { ReactNode } from "react";

export type RailProps = {
  /** Render a skeleton row instead of children. */
  loading?: boolean;
  /** Number of skeleton tiles while loading. */
  skeletonCount?: number;
  /** When true (and not loading), render `empty` instead of the children. */
  isEmpty?: boolean;
  /** Empty-state node, shown when `isEmpty`. */
  empty?: ReactNode;
  children?: ReactNode;
};

const SCROLL_ROW = "flex gap-4 overflow-x-auto pb-2";

function RailSkeleton({ count }: { count: number }) {
  return (
    <div className={SCROLL_ROW}>
      {Array.from({ length: count }).map((_, index) => (
        <div
          key={index}
          className="aspect-square w-48 shrink-0 animate-pulse rounded-[1.75rem] bg-base-300/70"
        />
      ))}
    </div>
  );
}

/**
 * Horizontal scrolling row shared by every Collections rail. Owns the scroll
 * container, the loading skeleton, and the empty-state slot so the individual
 * rails only declare their cards.
 */
export default function Rail({
  loading = false,
  skeletonCount = 4,
  isEmpty = false,
  empty = null,
  children,
}: RailProps) {
  if (loading) {
    return <RailSkeleton count={skeletonCount} />;
  }
  if (isEmpty) {
    return <>{empty}</>;
  }
  return <div className={SCROLL_ROW}>{children}</div>;
}

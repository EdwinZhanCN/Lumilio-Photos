import type { ReactNode } from "react";

/** A single `● LABEL` statistic. Shows a skeleton bar while `loading`. */
export function MetaStat({
  children,
  loading = false,
  skeletonWidth = "w-16",
}: {
  children?: ReactNode;
  loading?: boolean;
  skeletonWidth?: string;
}): ReactNode {
  return (
    <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
      <span className="text-[8px] text-primary">●</span>
      {loading ? (
        <div className={`h-3 ${skeletonWidth} animate-pulse rounded bg-base-300`} />
      ) : (
        <span>{children}</span>
      )}
    </div>
  );
}

/**
 * The dotted metadata strip shown under a collection detail title (items count,
 * date range, etc.). Previously hand-rolled and drifting between AlbumDetails,
 * TripDetails and PersonDetails — this is the single source. `dense` is the
 * compact variant used when a scrollable hero collapses.
 */
export function MetaStatRow({
  children,
  dense = false,
  className = "",
}: {
  children: ReactNode;
  dense?: boolean;
  className?: string;
}): ReactNode {
  return (
    <div
      className={`flex flex-wrap items-center gap-x-6 gap-y-1 transition-all duration-500 ease-in-out ${
        dense ? "text-[10px] opacity-60" : "text-xs opacity-40"
      } ${className}`}
    >
      {children}
    </div>
  );
}

export default MetaStatRow;

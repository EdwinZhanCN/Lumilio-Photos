import type { ReactNode } from "react";

/**
 * The big primary title used by collection detail heroes (album / trip /
 * person), with an optional mono "code" badge and a trailing slot for extra
 * badges or actions. Standardizes the `text-4xl font-black text-primary` look
 * that was copy-pasted across the detail routes.
 */
export function CollectionTitle({
  title,
  code,
  loading = false,
  children,
}: {
  title: ReactNode;
  /** e.g. "ALBUM #12" — rendered as a ghost mono badge. */
  code?: ReactNode;
  loading?: boolean;
  /** Extra badges / action buttons rendered after the code badge. */
  children?: ReactNode;
}): ReactNode {
  if (loading) {
    return <div className="h-10 w-64 animate-pulse rounded-lg bg-base-300" />;
  }
  return (
    <div className="flex flex-wrap items-baseline gap-4">
      <h1 className="truncate text-2xl sm:text-3xl lg:text-4xl font-black tracking-tight text-primary">
        {title}
      </h1>
      {code && <span className="badge badge-ghost font-mono text-xs opacity-50">{code}</span>}
      {children}
    </div>
  );
}

export default CollectionTitle;

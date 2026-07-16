import type { ReactNode } from "react";

/** Full-route fallback used by every lazy application route. */
export default function RouteLoadingFallback(): ReactNode {
  return (
    <div className="flex h-full items-center justify-center" role="status" aria-live="polite">
      <span className="loading loading-spinner loading-lg text-primary" />
    </div>
  );
}

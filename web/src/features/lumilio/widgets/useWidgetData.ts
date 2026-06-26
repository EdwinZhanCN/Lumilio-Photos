import { useMemo } from "react";
import type { AgentPinDTO, WidgetData, WidgetMode, WidgetSource } from "./types";
import { useWidgetMetadata } from "./useWidgetMetadata";

interface WidgetDataOverrides {
  /** Authoritative snapshot count (pin.count / block.count). Falls back to the
   * facet/metadata count when omitted. */
  count?: number;
  title?: string;
  /** Pins carry their own mode; inline chat results are treated as live. */
  mode?: WidgetMode;
}

/** Unified hydration: normalizes a ref/pin/mock source into the handoff's
 * WidgetData (count, title, mode, liveCount, facets, state). Thin composition
 * over useWidgetMetadata — views that need thumbnails fetch them separately via
 * useWidgetAssetsPreview, so Stat/Timeline never over-fetch images. */
export function useWidgetData(source: WidgetSource, overrides: WidgetDataOverrides = {}): WidgetData {
  const { metadata, facets, isLoading, isError } = useWidgetMetadata(source);

  return useMemo(() => {
    const pin = source.kind === "pin" ? (metadata as AgentPinDTO | undefined) : undefined;
    const mode: WidgetMode = overrides.mode ?? pin?.mode ?? "live";
    const count = overrides.count ?? metadata?.count ?? facets?.count ?? 0;
    // The facet summary reflects the set's current size; for a live pin that's
    // the replayed count to diff against the frozen snapshot.
    const liveCount = mode === "live" ? (facets?.count ?? undefined) : undefined;

    let state: WidgetData["state"];
    if (isLoading) state = "loading";
    else if (isError) state = "error";
    else if (count === 0) state = "empty";
    else state = "ready";

    return {
      count,
      title: overrides.title ?? undefined,
      mode,
      liveCount,
      facets,
      state,
    };
  }, [source.kind, metadata, facets, isLoading, isError, overrides.count, overrides.title, overrides.mode]);
}

import type { ComponentType } from "react";
import type { LucideIcon } from "lucide-react";
import type { components } from "@/lib/http-commons/schema";

export type AgentPinDTO = components["schemas"]["dto.AgentPinDTO"];
export type AgentRefFacetsDTO = components["schemas"]["dto.AgentRefFacetsDTO"];

/** Where a widget's assets hydrate from: a session ref (chat) or a durable
 * pin (board). View bodies never receive asset data directly — they fetch it
 * themselves through the hydration hooks (useWidgetData / useWidgetAssets). */
export type WidgetSource =
  | { kind: "ref"; refId: string; threadId: string }
  | { kind: "pin"; pinId: string }
  | { kind: "mock"; mockId: string };

/** Discrete board size tier. Cells don't free-resize — every cell sits at one
 * of these tiers (chosen via the cell menu), and each view styles itself by
 * tier rather than reacting to arbitrary pixel sizes. */
export type WidgetSizeKey = "s" | "m" | "l";

/** Whether a pinned set keeps replaying its query (live) or is a fixed
 * snapshot (frozen). Inline chat results are treated as live. */
export type WidgetMode = "live" | "frozen";

/** Per-view loading/error/empty state, resolved once by the tile shell and
 * shared across every view body. */
export type WidgetState = "ready" | "loading" | "error" | "empty";

/** The normalized data every View receives — the handoff's WidgetData. Sourced
 * from the ref/pin facet APIs by useWidgetData; identical shape for chat refs
 * and board pins so views are source-agnostic. */
export interface WidgetData {
  /** Total members of the underlying set, as captured when pinned/shown. */
  count: number;
  title?: string;
  mode: WidgetMode;
  /** Live mode only: the current replayed count. delta = max(0, liveCount - count). */
  liveCount?: number;
  facets?: AgentRefFacetsDTO;
  state: WidgetState;
}

/** `+N` growth a live set has accrued since it was pinned. */
export function widgetDelta(data: Pick<WidgetData, "mode" | "count" | "liveCount">): number {
  if (data.mode !== "live" || data.liveCount == null) return 0;
  return Math.max(0, data.liveCount - data.count);
}

/** Props a pure View body renders from. The tile shell owns chrome, state
 * gating and the deep-link; the body only paints the "ready" presentation. */
export interface ViewBodyProps {
  data: WidgetData;
  size: WidgetSizeKey;
  /** "glass" when painted under a floating glass header (Cover on the board). */
  ctx: "board" | "inline" | "glass";
  source: WidgetSource;
}

/** A react-grid-layout cell footprint. */
export interface WidgetLayout {
  w: number;
  h: number;
  minW?: number;
  minH?: number;
}

/** A view registration. Adding a view = registering one of these; the View
 * switcher, chat blocks, the pin flow and the board all render through the
 * registry. A "widget" is just the currently selected view over a pinned ref,
 * so every view shares one footprint per tier (see DIMS in registry). */
export interface WidgetDefinition {
  type: string;
  /** Pure presentational body for the "ready" state. */
  View: ComponentType<ViewBodyProps>;
  /** lucide icon shown in the View switcher segmented control. The view's
   * localized name is resolved in ViewSwitcher (static t() calls, by type). */
  icon: LucideIcon;
  /** Default react-grid-layout cell for newly pinned widgets (the shared M
   * footprint). */
  defaultLayout: WidgetLayout;
}

import { BarChart3, Grid2x2, Hash, Image } from "lucide-react";
import { CoverView } from "./views/CoverView";
import { StatView } from "./views/StatView";
import { TimelineView } from "./views/TimelineView";
import { MosaicView } from "./views/MosaicView";
import type { WidgetDefinition, WidgetSizeKey } from "./types";

/** Widget registry: extending the system = adding one WidgetDefinition here
 * (plus a backend widget type string). Unknown types degrade to a hint. Every
 * registration is also a selectable View — its label/icon drive the switcher.
 * Unlike before, footprint is NOT per-view: all views share one footprint per
 * size tier (DIMS), so switching view never reshapes the cell. */
const registry = new Map<string, WidgetDefinition>();

export function registerWidget(definition: WidgetDefinition) {
  registry.set(definition.type, definition);
}

export function getWidget(type: string): WidgetDefinition | undefined {
  return registry.get(type);
}

export function listWidgets(): WidgetDefinition[] {
  return Array.from(registry.values());
}

/** One shared footprint per tier — identical for every view (cols × rows on
 * the 12-col / 72px grid). Handoff §Size Tiers. */
export const DIMS: Record<WidgetSizeKey, { w: number; h: number }> = {
  s: { w: 3, h: 3 },
  m: { w: 4, h: 4 },
  l: { w: 6, h: 5 },
};

/** Min cell a widget may occupy (the S footprint). */
export const WIDGET_MIN = { minW: DIMS.s.w, minH: DIMS.s.h } as const;

const SIZE_KEYS: WidgetSizeKey[] = ["s", "m", "l"];

/** Which S/M/L tier a cell footprint matches. View-independent now that every
 * view shares DIMS; legacy/odd sizes fall back to "m". */
export function sizeKeyFor(w: number, h: number): WidgetSizeKey {
  for (const key of SIZE_KEYS) {
    if (DIMS[key].w === w && DIMS[key].h === h) return key;
  }
  return "m";
}

const DEFAULT_LAYOUT = { w: DIMS.m.w, h: DIMS.m.h, ...WIDGET_MIN };

registerWidget({ type: "cover_card", View: CoverView, icon: Image, defaultLayout: DEFAULT_LAYOUT });
registerWidget({ type: "number_card", View: StatView, icon: Hash, defaultLayout: DEFAULT_LAYOUT });
registerWidget({
  type: "spark_card",
  View: TimelineView,
  icon: BarChart3,
  defaultLayout: DEFAULT_LAYOUT,
});
registerWidget({
  type: "mosaic_card",
  View: MosaicView,
  icon: Grid2x2,
  defaultLayout: DEFAULT_LAYOUT,
});

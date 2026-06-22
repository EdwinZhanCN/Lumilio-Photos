import type { WidgetDefinition } from "./types";
import { AssetGridWidget } from "./AssetGridWidget";
import { FacetDashboardWidget } from "./FacetDashboardWidget";
import { StorylineWidget } from "./StorylineWidget";
import { TimelineWidget } from "./TimelineWidget";

/** Widget registry: extending the system = adding one WidgetDefinition here
 * (plus a backend widget type string). Unknown types degrade to a hint. */
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

registerWidget({
  type: "asset_grid",
  Component: AssetGridWidget,
  defaultLayout: { w: 4, h: 4, minW: 2, minH: 2 },
});

registerWidget({
  type: "facet_dashboard",
  Component: FacetDashboardWidget,
  defaultLayout: { w: 4, h: 4, minW: 3, minH: 3 },
});

registerWidget({
  type: "timeline",
  Component: TimelineWidget,
  defaultLayout: { w: 4, h: 3, minW: 3, minH: 2 },
});

registerWidget({
  type: "storyline",
  Component: StorylineWidget,
  defaultLayout: { w: 4, h: 4, minW: 3, minH: 3 },
});

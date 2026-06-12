import type { WidgetDefinition } from "./types";
import { AssetGridWidget } from "./AssetGridWidget";

/** Widget registry: extending the system = adding one WidgetDefinition here
 * (plus a backend widget type string). Unknown types degrade to a hint. */
const registry = new Map<string, WidgetDefinition>();

export function registerWidget(definition: WidgetDefinition) {
  registry.set(definition.type, definition);
}

export function getWidget(type: string): WidgetDefinition | undefined {
  return registry.get(type);
}

registerWidget({
  type: "asset_grid",
  Component: AssetGridWidget,
  defaultLayout: { w: 4, h: 4, minW: 2, minH: 2 },
});

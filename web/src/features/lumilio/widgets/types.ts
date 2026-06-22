import type { ComponentType } from "react";
import type { components } from "@/lib/http-commons/schema";

export type AgentPinDTO = components["schemas"]["dto.AgentPinDTO"];

/** Where a widget's assets hydrate from: a session ref (chat) or a durable
 * pin (board). Widget components never receive asset data directly — they
 * fetch it themselves through the hydration APIs. */
export type WidgetSource =
  | { kind: "ref"; refId: string; threadId: string }
  | { kind: "pin"; pinId: string }
  | { kind: "mock"; mockId: string };

/** Render contexts: inline (inside a chat message) renders a compact
 * preview; board fills its react-grid-layout cell. */
export type WidgetVariant = "inline" | "board";

export interface WidgetProps {
  source: WidgetSource;
  variant: WidgetVariant;
  /** Total members of the underlying set, as known when rendered. */
  count: number;
  title?: string;
  params?: Record<string, unknown>;
}

/** A widget type registration. Adding a widget = registering one of these;
 * chat blocks, the pin flow and the board all render through the registry. */
export interface WidgetDefinition {
  type: string;
  Component: ComponentType<WidgetProps>;
  /** Default react-grid-layout cell for newly pinned widgets. */
  defaultLayout: { w: number; h: number; minW?: number; minH?: number };
}

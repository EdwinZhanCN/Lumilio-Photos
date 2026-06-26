import { useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { fmt } from "../format";
import { getWidget } from "../registry";
import { PinButton } from "../PinButton";
import type { WidgetSource } from "../types";
import { useWidgetData } from "../useWidgetData";
import { EmptyState, ErrorState, LoadingState } from "../views/states";
import { LiveBadge } from "./LiveBadge";
import { ViewSwitcher } from "./ViewSwitcher";

interface InlineWidgetCardProps {
  refId: string;
  threadId: string;
  /** Agent's chosen initial view; the user can switch it locally before pinning. */
  widget: string;
  count: number;
  title?: string;
}

/** The same widget rendered inline in the chat stream: a compact card with a
 * solid header, a fixed-height body, and a footer carrying the View switcher
 * and the Pin action. Timeline renders at its L treatment here for legibility;
 * other views render at M. */
export function InlineWidgetCard({ refId, threadId, widget, count, title }: InlineWidgetCardProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const [view, setView] = useState(widget);
  const definition = getWidget(view);
  const source: WidgetSource = { kind: "ref", refId, threadId };
  const data = useWidgetData(source, { count, title, mode: "live" });

  if (!definition) {
    return (
      <div className="my-2 text-xs text-base-content/50">
        {t("lumilio.board.unknownWidget", "Unknown widget type: {{type}}", { type: view })}
      </div>
    );
  }

  const View = definition.View;
  const size = view === "spark_card" ? "l" : "m";

  let body;
  if (data.state === "loading") body = <LoadingState view={view} size={size} />;
  else if (data.state === "error") body = <ErrorState size={size} />;
  else if (data.state === "empty") body = <EmptyState size={size} />;
  else body = <View data={data} size={size} ctx="inline" source={source} />;

  return (
    <div className="card group my-3 flex w-full max-w-sm flex-col overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm">
      <div className="flex h-10 shrink-0 items-center gap-1.5 border-b border-base-200 bg-base-100 px-2.5">
        <div className="flex min-w-0 flex-1 items-center gap-1.5">
          <span className="truncate font-bold text-base-content">
            {data.title || t("lumilio.board.untitled", "Untitled")}
          </span>
          <span className="shrink-0 text-xs tabular-nums text-base-content/45">
            {fmt(data.count, locale)}
          </span>
          <LiveBadge data={data} size="m" locale={locale} />
        </div>
      </div>
      <div className="relative flex h-44 flex-col">{body}</div>
      <div className="flex items-center justify-between gap-2 border-t border-base-200 bg-base-100 px-2.5 py-2">
        <ViewSwitcher current={view} onChange={setView} size="xs" />
        <PinButton refId={refId} threadId={threadId} widget={view} title={data.title} />
      </div>
    </div>
  );
}

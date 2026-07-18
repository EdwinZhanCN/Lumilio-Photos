import { Snowflake } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { fmt } from "../format";
import { widgetDelta, type WidgetData, type WidgetSizeKey } from "../types";

interface LiveBadgeProps {
  data: Pick<WidgetData, "mode" | "count" | "liveCount">;
  size: WidgetSizeKey;
  /** Locale for the +N delta. */
  locale?: string;
}

/** Live / frozen indicator. Live: a pulsing success dot (S = just the dot;
 * M = dot + "+N"; L = dot + "Live" + "+N"). Frozen: a snowflake (L = with
 * "Frozen", M = glyph only, S = nothing). */
export function LiveBadge({ data, size, locale }: LiveBadgeProps) {
  const { t } = useI18n();
  const delta = widgetDelta(data);

  if (data.mode === "live") {
    if (size === "s") {
      return (
        <span className="lumilio-livedot inline-flex h-2 w-2 rounded-full bg-success ring-2 ring-base-100" />
      );
    }
    return (
      <span className="badge badge-sm gap-1 border-0 bg-success/15 font-bold text-success">
        <span className="lumilio-livedot inline-flex h-1.5 w-1.5 rounded-full bg-success" />
        {size === "l" && t("lumilio.widgets.live", "Live")}
        {delta > 0 && `+${fmt(delta, locale)}`}
      </span>
    );
  }

  if (size === "l") {
    return (
      <span className="badge badge-ghost badge-sm gap-1 text-base-content/55">
        <Snowflake size={12} strokeWidth={1.85} />
        {t("lumilio.widgets.frozen", "Frozen")}
      </span>
    );
  }
  if (size === "m") {
    return (
      <span className="text-base-content/35">
        <Snowflake size={14} strokeWidth={1.85} />
      </span>
    );
  }
  return null;
}

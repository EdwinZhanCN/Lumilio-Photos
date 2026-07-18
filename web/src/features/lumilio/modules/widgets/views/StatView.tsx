import { Heart, ImageIcon, Video } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { fmt, typeCount } from "../format";
import { widgetDelta, type ViewBodyProps, type WidgetSizeKey } from "../types";

const NUMBER_SIZE: Record<WidgetSizeKey, string> = {
  s: "text-[2rem]",
  m: "text-4xl",
  l: "text-5xl",
};

/** Stat — a pure statistic: one big number + label. Live sets show a "+N new"
 * delta below the number (kept on its own line so the number stays centered).
 * L adds a 3-tile footer (liked / images / videos). */
export function StatView({ data, size }: ViewBodyProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const facets = data.facets;
  const delta = widgetDelta(data);
  const isLikedSet = !!data.title && /liked|喜欢|收藏/i.test(data.title);

  const main = (
    <div className="flex flex-1 flex-col items-center justify-center gap-1.5 px-2 text-center">
      <span
        className={`font-extrabold leading-none tracking-tight tabular-nums text-base-content ${NUMBER_SIZE[size]}`}
      >
        {fmt(data.count, locale)}
      </span>
      {delta > 0 && (
        <span className="badge badge-sm border-0 bg-success/15 font-bold text-success">
          {t("lumilio.widgets.stat.newDelta", "+{{count}} new", { count: delta })}
        </span>
      )}
      <div
        className={`font-semibold uppercase tracking-wide text-base-content/45 ${
          size === "s" ? "text-[10px]" : "text-xs"
        }`}
      >
        {isLikedSet
          ? t("lumilio.widgets.stat.likedLabel", "liked photos")
          : t("lumilio.widgets.photos", "photos")}
      </div>
    </div>
  );

  if (size !== "l") return main;

  const tiles: { icon: typeof Heart; value: number; caption: string }[] = [
    {
      icon: Heart,
      value: facets?.liked_count ?? 0,
      caption: t("lumilio.widgets.stat.liked", "liked"),
    },
    {
      icon: ImageIcon,
      value: typeCount(facets?.types, "image") || data.count,
      caption: t("lumilio.widgets.stat.images", "images"),
    },
    {
      icon: Video,
      value: typeCount(facets?.types, "video"),
      caption: t("lumilio.widgets.stat.videos", "videos"),
    },
  ];

  return (
    <div className="flex flex-1 flex-col">
      {main}
      <div className="grid grid-cols-3 divide-x divide-base-200 border-t border-base-200">
        {tiles.map(({ icon: Icon, value, caption }) => (
          <div key={caption} className="flex flex-col items-center justify-center gap-0.5 py-2.5">
            <Icon className="text-base-content/40" size={16} strokeWidth={1.75} />
            <span className="text-sm font-bold tabular-nums text-base-content">
              {fmt(value, locale)}
            </span>
            <span className="text-[10px] text-base-content/45">{caption}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

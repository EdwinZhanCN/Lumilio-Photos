import { Calendar, Heart, ImageIcon, MapPin, Video } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { dateRangeLabel, fmt, typeCount } from "../format";
import type { ViewBodyProps } from "../types";
import { useWidgetAssetsPreview } from "../useWidgetAssets";
import { WidgetAssetThumbnail } from "../WidgetAssetThumbnail";

/** Cover — one representative photo summarizing the set. Fills the body with
 * the photo and floats a gradient caption; the size tier decides how much of
 * the caption (chips, date/place meta) shows. */
export function CoverView({ data, size, source }: ViewBodyProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const { assets, isLoading } = useWidgetAssetsPreview(source, 1);
  const cover = assets[0];

  const heading = data.title ?? t("lumilio.widgets.resultsTitle", "Photos from Lumilio");
  const facets = data.facets;
  const date = dateRangeLabel(facets?.date_range);
  const place = facets?.top_places?.[0]?.name;
  const person = facets?.top_people?.[0]?.name;
  const videoCount = typeCount(facets?.types, "video");
  const likedCount = facets?.liked_count ?? 0;

  const pad = size === "s" ? "p-2" : "p-3";
  const titleSize = size === "l" ? "text-2xl" : size === "m" ? "text-lg" : "text-sm";
  const metaSize = size === "l" ? "text-sm" : "text-xs";

  return (
    <div className="absolute inset-0 bg-base-300">
      {isLoading ? (
        <div className="skeleton absolute inset-0" />
      ) : cover ? (
        <WidgetAssetThumbnail
          asset={cover}
          source={source}
          className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.04]"
        />
      ) : (
        <div className="absolute inset-0 grid place-items-center text-base-content/25">
          <ImageIcon size={28} strokeWidth={1.5} />
        </div>
      )}
      <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent" />
      <div className={`absolute inset-x-0 bottom-0 text-white ${pad}`}>
        {size === "l" && (person || likedCount > 0 || videoCount > 0) && (
          <div className="mb-1.5 flex flex-wrap gap-1">
            {person && (
              <span className="badge badge-sm border-0 bg-white/20 text-white backdrop-blur-sm">
                {person}
              </span>
            )}
            {likedCount > 0 && (
              <span className="badge badge-sm gap-1 border-0 bg-white/20 text-white backdrop-blur-sm">
                <Heart size={12} strokeWidth={2} />
                {fmt(likedCount, locale)}
              </span>
            )}
            {videoCount > 0 && (
              <span className="badge badge-sm gap-1 border-0 bg-white/20 text-white backdrop-blur-sm">
                <Video size={12} strokeWidth={2} />
                {fmt(videoCount, locale)}
              </span>
            )}
          </div>
        )}
        <div className={`truncate font-extrabold leading-tight drop-shadow ${titleSize}`}>
          {heading}
        </div>
        <div
          className={`mt-0.5 flex items-center gap-2.5 font-medium text-white/85 ${metaSize}`}
        >
          <span className="font-semibold tabular-nums">
            {size === "s"
              ? fmt(data.count, locale)
              : `${fmt(data.count, locale)} ${t("lumilio.widgets.photos", "photos")}`}
          </span>
          {size !== "s" && date && (
            <span className="inline-flex items-center gap-1">
              <Calendar size={12} strokeWidth={2} />
              {date}
            </span>
          )}
          {size !== "s" && place && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <MapPin size={12} strokeWidth={2} />
              <span className="truncate">{place}</span>
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

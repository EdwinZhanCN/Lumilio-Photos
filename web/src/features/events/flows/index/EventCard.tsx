import { CalendarDays, Images } from "lucide-react";
import { Link } from "react-router-dom";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { eventDateRange, eventTitle, type EventSummary } from "../../model/event";

export function EventCard({ event }: { event: EventSummary }) {
  const { t, i18n } = useI18n();
  return (
    <Link
      to={`/collections/events/${event.event_id}`}
      className="card card-border overflow-hidden bg-base-100 transition hover:border-base-content/25"
    >
      <figure className="aspect-[4/3] bg-base-200">
        {event.cover_asset_id ? (
          <img
            src={assetUrls.getThumbnailUrl(event.cover_asset_id, "medium")}
            alt=""
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-base-content/35">
            <Images className="size-10" strokeWidth={1.25} />
            <span className="sr-only">{t("events.noCover", "No displayable cover")}</span>
          </div>
        )}
      </figure>
      <div className="card-body gap-2 p-4">
        <h2 className="card-title line-clamp-1 text-base">{eventTitle(event, t)}</h2>
        <div className="flex items-center gap-2 text-sm text-base-content/65">
          <CalendarDays className="size-4" />
          <span>{eventDateRange(event, i18n.resolvedLanguage)}</span>
        </div>
        <div className="text-sm text-base-content/55">
          {t("events.mediaCount", "{{count}} media").replace(
            "{{count}}",
            String(event.media_count),
          )}
        </div>
      </div>
    </Link>
  );
}

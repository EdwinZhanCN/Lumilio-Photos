import { CalendarRange } from "lucide-react";
import { useNavigate } from "react-router-dom";
import RailCard from "@/components/collection/RailCard";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { eventDateRange, eventTitle, type EventSummary } from "../../model/event";

export function EventCard({ event }: { event: EventSummary }) {
  const { t, i18n } = useI18n();
  const navigate = useNavigate();
  const mediaCount = t("events.mediaCount", "{{count}} media").replace(
    "{{count}}",
    String(event.media_count),
  );
  const subtitle = [
    eventDateRange(event, i18n.resolvedLanguage),
    mediaCount,
    event.is_hidden ? t("events.hiddenBadge", "Hidden") : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <RailCard
      media={{
        kind: "photo",
        src: event.cover_asset_id
          ? assetUrls.getThumbnailUrl(event.cover_asset_id, "medium")
          : null,
        fallbackIcon: CalendarRange,
      }}
      title={eventTitle(event, t)}
      subtitle={subtitle}
      onClick={event.event_id ? () => navigate(`/collections/events/${event.event_id}`) : undefined}
      className="w-full"
    />
  );
}

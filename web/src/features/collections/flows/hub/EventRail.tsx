import { CalendarRange } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { eventDateRange, eventTitle, type EventSummary } from "@/features/events";
import Rail from "../../components/Rail";
import RailCard from "@/components/collection/RailCard";

type EventRailProps = {
  events: EventSummary[];
  loading?: boolean;
  onEventClick?: (event: EventSummary) => void;
};

export default function EventRail({ events, loading = false, onEventClick }: EventRailProps) {
  const { t, i18n } = useI18n();

  return (
    <Rail
      loading={loading}
      isEmpty={events.length === 0}
      empty={
        <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
          {t("events.emptyTitle", "No Events yet")}
        </div>
      }
    >
      {events.map((event) => {
        const mediaCount = t("events.mediaCount", "{{count}} media").replace(
          "{{count}}",
          String(event.media_count),
        );

        return (
          <RailCard
            key={event.event_id}
            media={{
              kind: "photo",
              src: event.cover_asset_id
                ? assetUrls.getThumbnailUrl(event.cover_asset_id, "medium")
                : null,
              fallbackIcon: CalendarRange,
            }}
            title={eventTitle(event, t)}
            subtitle={`${eventDateRange(event, i18n.resolvedLanguage)} · ${mediaCount}`}
            onClick={() => onEventClick?.(event)}
            className="w-48"
          />
        );
      })}
    </Rail>
  );
}

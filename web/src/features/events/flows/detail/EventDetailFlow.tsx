import { CalendarRange } from "lucide-react";
import { Navigate, useParams } from "react-router-dom";
import { AssetBrowser, AssetBrowserScope } from "@/features/assets";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { useEvent } from "../../api/useEvents";
import type { EventDetail } from "../../model/event";
import { eventTitle } from "../../model/event";
import EventHero from "./components/EventHero";
import { useEventBulkActions } from "./useEventBulkActions";

type EventQuery = ReturnType<typeof useEvent>;

function EventGallery({ event, eventQuery }: { event: EventDetail; eventQuery: EventQuery }) {
  const { t } = useI18n();
  const { bulkActions, dialogs } = useEventBulkActions(event, eventQuery);

  return (
    <AssetBrowserScope
      scopeId={`event:${event.event_id}`}
      basePath={`/collections/events/${event.event_id}`}
    >
      <AssetBrowser
        title={eventTitle(event, t)}
        icon={<CalendarRange className="size-6 text-primary" strokeWidth={1.5} />}
        constraint={{ event_id: event.event_id }}
        hero={
          <EventHero
            event={event}
            isPatching={eventQuery.isPatching}
            isMerging={eventQuery.isMerging}
            isSharing={eventQuery.isSharing}
            isAdding={eventQuery.isAdding}
            onPatch={eventQuery.patch}
            onMerge={eventQuery.merge}
            onShare={eventQuery.share}
            onAdd={eventQuery.addAssets}
          />
        }
        bulkActions={bulkActions}
        hiddenBulkActions={["delete-assets"]}
        viewKey={`event:${event.event_id}`}
      />
      {dialogs}
    </AssetBrowserScope>
  );
}

export function EventDetailFlow() {
  const { eventId } = useParams<{ eventId: string }>();
  const { t } = useI18n();
  const eventQuery = useEvent(eventId);
  const event = eventQuery.data;

  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("events.title", "Events"), to: "/collections/events" },
    { label: event ? eventTitle(event, t) : t("events.generatedTitle", "Event") },
  ]);

  if (!eventId) return <Navigate to="/collections/events" replace />;
  if (eventQuery.isPending) {
    return (
      <div className="flex h-full flex-col">
        <div className="border-b border-base-200 px-4 py-3">
          <div className="skeleton h-6 w-48" />
        </div>
        <div className="space-y-4 p-4">
          <div className="flex items-center gap-4">
            <div className="skeleton size-20 rounded-[1.5rem]" />
            <div className="space-y-2">
              <div className="skeleton h-6 w-52" />
              <div className="skeleton h-3 w-36" />
            </div>
          </div>
          <div className="grid grid-cols-3 gap-1 sm:grid-cols-5">
            {Array.from({ length: 10 }, (_, index) => (
              <div key={index} className="skeleton aspect-square" />
            ))}
          </div>
        </div>
      </div>
    );
  }
  if (eventQuery.isError || !event) {
    return (
      <div role="alert" className="alert alert-error alert-soft m-6">
        {t("events.detailError", "This Event could not be loaded.")}
      </div>
    );
  }

  return <EventGallery event={event} eventQuery={eventQuery} />;
}

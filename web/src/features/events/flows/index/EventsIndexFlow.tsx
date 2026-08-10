import { CalendarRange } from "lucide-react";
import { useState } from "react";
import PageHeader from "@/components/ui/PageHeader";
import EmptyState from "@/components/ui/EmptyState";
import { CollectionErrorAlert, LoadMoreButton } from "@/components/collection";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { BrowseScopeSelect } from "@/features/repositories";
import { useI18n } from "@/lib/i18n";
import { useEvents } from "../../api/useEvents";
import { EventCard } from "./EventCard";

export function EventsIndexFlow() {
  const { t } = useI18n();
  const [includeHidden, setIncludeHidden] = useState(false);
  const query = useEvents({ includeHidden });
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("events.title", "Events") },
  ]);
  const events = query.data?.pages.flatMap((page) => page.events ?? []) ?? [];

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("events.title", "Events")}
        icon={<CalendarRange className="size-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
        <div className="join">
          <button
            type="button"
            className={`btn btn-sm join-item ${includeHidden ? "btn-ghost" : "btn-active"}`}
            onClick={() => setIncludeHidden(false)}
          >
            {t("events.visibleTab", "Visible")}
          </button>
          <button
            type="button"
            className={`btn btn-sm join-item ${includeHidden ? "btn-active" : "btn-ghost"}`}
            onClick={() => setIncludeHidden(true)}
          >
            {t("events.allTab", "All")}
          </button>
        </div>
      </PageHeader>

      <main className="min-h-0 flex-1 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {query.isError && (
            <CollectionErrorAlert message={t("events.loadError", "Events could not be loaded.")} />
          )}

          {query.isPending && (
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
              {Array.from({ length: 10 }, (_, index) => (
                <div
                  key={index}
                  className="aspect-square animate-pulse rounded-[1.75rem] bg-base-300/70"
                />
              ))}
            </div>
          )}

          {!query.isPending && !query.isError && events.length === 0 && (
            <EmptyState
              title={t("events.emptyTitle", "No Events yet")}
              description={t(
                "events.emptyDescription",
                "Rebuild to organize your media by time and place.",
              )}
            />
          )}

          {events.length > 0 && (
            <>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                {events.map((event) => (
                  <EventCard key={event.event_id} event={event} />
                ))}
              </div>

              {query.hasNextPage && (
                <LoadMoreButton
                  onClick={() => void query.fetchNextPage()}
                  loading={query.isFetchingNextPage}
                />
              )}
            </>
          )}
        </div>
      </main>
    </div>
  );
}

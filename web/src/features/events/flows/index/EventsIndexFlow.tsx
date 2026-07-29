import { CalendarRange, RefreshCw } from "lucide-react";
import PageHeader from "@/components/ui/PageHeader";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { useEventRebuild, useEvents } from "../../api/useEvents";
import { EventCard } from "./EventCard";

export function EventsIndexFlow() {
  const { t } = useI18n();
  const query = useEvents();
  const { rebuild, isRebuilding } = useEventRebuild();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("events.title", "Events") },
  ]);
  const events = query.data?.pages.flatMap((page) => page.events ?? []) ?? [];
  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("events.title", "Events")} icon={<CalendarRange className="size-6" />}>
        <button
          type="button"
          className="btn btn-sm"
          disabled={isRebuilding}
          onClick={() => void rebuild(false)}
        >
          <RefreshCw className={`size-4 ${isRebuilding ? "animate-spin" : ""}`} />
          {t("events.rebuild", "Rebuild")}
        </button>
      </PageHeader>
      <main className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
        {query.isPending && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {Array.from({ length: 8 }, (_, index) => (
              <div key={index} className="skeleton aspect-[4/3]" />
            ))}
          </div>
        )}
        {query.isError && (
          <div role="alert" className="alert alert-error alert-soft">
            {t("events.loadError", "Events could not be loaded.")}
          </div>
        )}
        {!query.isPending && !query.isError && events.length === 0 && (
          <div className="hero min-h-72 rounded-box bg-base-200">
            <div className="hero-content text-center">
              <div>
                <CalendarRange className="mx-auto mb-4 size-10 text-base-content/40" />
                <h2 className="text-xl font-semibold">{t("events.emptyTitle", "No Events yet")}</h2>
                <p className="mt-2 text-base-content/60">
                  {t(
                    "events.emptyDescription",
                    "Rebuild to organize your media by time and place.",
                  )}
                </p>
              </div>
            </div>
          </div>
        )}
        {events.length > 0 && (
          <>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {events.map((event) => (
                <EventCard key={event.event_id} event={event} />
              ))}
            </div>
            {query.hasNextPage && (
              <div className="mt-6 flex justify-center">
                <button
                  type="button"
                  className="btn"
                  disabled={query.isFetchingNextPage}
                  onClick={() => void query.fetchNextPage()}
                >
                  {t("common.loadMore", "Load more")}
                </button>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
}

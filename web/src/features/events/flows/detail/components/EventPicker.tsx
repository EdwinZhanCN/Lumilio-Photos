import { useMemo, useState } from "react";
import { CalendarRange, Check, Search } from "lucide-react";
import { LoadMoreButton } from "@/components/collection";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { useEvents } from "../../../api/useEvents";
import { eventDateRange, eventTitle } from "../../../model/event";

type EventPickerProps = {
  selectedIds: string[];
  onChange: (ids: string[]) => void;
  excludeIds?: string[];
  multiSelect?: boolean;
};

/** Owner-wide Event picker used by merge and move correction flows. */
export default function EventPicker({
  selectedIds,
  onChange,
  excludeIds = [],
  multiSelect = false,
}: EventPickerProps) {
  const { t, i18n } = useI18n();
  const [search, setSearch] = useState("");
  const query = useEvents({ limit: 100, followBrowseScope: false, includeHidden: true });
  const events = query.data?.pages.flatMap((page) => page.events ?? []) ?? [];
  const excluded = useMemo(() => new Set(excludeIds), [excludeIds]);
  const filtered = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase();
    return events.filter((event) => {
      if (!event.event_id || excluded.has(event.event_id)) return false;
      if (!needle) return true;
      return [
        eventTitle(event, t),
        eventDateRange(event, i18n.resolvedLanguage),
        event.event_id,
      ].some((value) => value.toLocaleLowerCase().includes(needle));
    });
  }, [events, excluded, i18n.resolvedLanguage, search, t]);

  const toggle = (eventId: string) => {
    if (multiSelect) {
      onChange(
        selectedIds.includes(eventId)
          ? selectedIds.filter((id) => id !== eventId)
          : [...selectedIds, eventId],
      );
      return;
    }
    onChange(selectedIds.includes(eventId) ? [] : [eventId]);
  };

  return (
    <div className="space-y-3">
      <label className="input input-bordered flex items-center gap-2">
        <Search className="size-4 text-base-content/50" />
        <input
          type="search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t("events.picker.searchPlaceholder", "Search Events")}
          className="grow"
        />
      </label>

      <div className="max-h-80 space-y-1 overflow-y-auto pr-1">
        {query.isPending ? (
          <div className="space-y-2 py-1">
            {Array.from({ length: 4 }, (_, index) => (
              <div key={index} className="flex items-center gap-3 rounded-xl px-3 py-2">
                <div className="skeleton size-12 rounded-xl" />
                <div className="flex-1 space-y-2">
                  <div className="skeleton h-3 w-36" />
                  <div className="skeleton h-2.5 w-52" />
                </div>
              </div>
            ))}
          </div>
        ) : query.isError ? (
          <div role="alert" className="alert alert-error alert-soft text-sm">
            {t("events.picker.loadError", "Events could not be loaded.")}
          </div>
        ) : filtered.length === 0 ? (
          <div className="py-8 text-center text-sm text-base-content/50">
            {t("events.picker.empty", "No matching Events")}
          </div>
        ) : (
          filtered.map((event) => {
            const eventId = event.event_id ?? "";
            const selected = selectedIds.includes(eventId);
            const coverUrl = event.cover_asset_id
              ? assetUrls.getThumbnailUrl(event.cover_asset_id, "small")
              : null;
            return (
              <button
                key={eventId}
                type="button"
                onClick={() => toggle(eventId)}
                className={`flex w-full items-center gap-3 rounded-xl border px-3 py-2 text-left transition-colors ${
                  selected ? "border-primary bg-primary/10" : "border-transparent hover:bg-base-200"
                }`}
              >
                <span className="size-12 shrink-0 overflow-hidden rounded-xl bg-base-200">
                  {coverUrl ? (
                    <img
                      src={coverUrl}
                      alt={eventTitle(event, t)}
                      className="size-full object-cover"
                    />
                  ) : (
                    <span className="flex size-full items-center justify-center">
                      <CalendarRange className="size-5 text-base-content/35" strokeWidth={1.5} />
                    </span>
                  )}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{eventTitle(event, t)}</span>
                  <span className="block truncate text-xs text-base-content/55">
                    {eventDateRange(event, i18n.resolvedLanguage)} ·{" "}
                    {t("events.mediaCount", "{{count}} media").replace(
                      "{{count}}",
                      String(event.media_count ?? 0),
                    )}
                  </span>
                </span>
                {selected && <Check className="size-4 shrink-0 text-primary" />}
              </button>
            );
          })
        )}
      </div>

      {query.hasNextPage && (
        <LoadMoreButton
          onClick={() => void query.fetchNextPage()}
          loading={query.isFetchingNextPage}
        />
      )}
    </div>
  );
}

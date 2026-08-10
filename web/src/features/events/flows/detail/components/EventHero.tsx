import { useState } from "react";
import { CalendarRange, EyeOff, Plus, Share2 } from "lucide-react";
import { CollectionHero, MetaStat } from "@/components/collection";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import type { EventDetail, EventPatch, EventShareRequest } from "../../../model/event";
import { eventDateRange, eventTitle } from "../../../model/event";
import EventAddMediaModal from "./EventAddMediaModal";
import EventEditModal from "./EventEditModal";
import EventShareModal from "./EventShareModal";

type EventHeroProps = {
  event: EventDetail;
  isPatching: boolean;
  isMerging: boolean;
  isSharing: boolean;
  isAdding: boolean;
  onPatch: (patch: EventPatch) => Promise<unknown>;
  onMerge: (eventIds: string[]) => Promise<unknown>;
  onShare: (request: EventShareRequest) => Promise<{ token?: string }>;
  onAdd: (assetIds: string[]) => Promise<unknown>;
};

/** Event metadata surface composed with the same collection language as People and Albums. */
export default function EventHero({
  event,
  isPatching,
  isMerging,
  isSharing,
  isAdding,
  onPatch,
  onMerge,
  onShare,
  onAdd,
}: EventHeroProps) {
  const { t, i18n } = useI18n();
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [isShareOpen, setIsShareOpen] = useState(false);
  const [isAddOpen, setIsAddOpen] = useState(false);
  const title = eventTitle(event, t);
  const coverUrl = event.cover_asset_id
    ? assetUrls.getThumbnailUrl(event.cover_asset_id, "medium")
    : null;
  const shareTooLarge = (event.displayable_count ?? 0) > 5000;

  const cover = (
    <div className="size-20 overflow-hidden rounded-[1.5rem] border border-base-300/70 bg-base-200">
      {coverUrl ? (
        <img src={coverUrl} alt={title} className="size-full object-cover" />
      ) : (
        <div className="flex size-full items-center justify-center">
          <CalendarRange className="size-8 text-base-content/35" strokeWidth={1.5} />
        </div>
      )}
    </div>
  );

  return (
    <>
      <CollectionHero
        cover={cover}
        title={title}
        code={t("events.detailCode", "EVENT {{id}}", {
          id: event.event_id?.slice(0, 8).toUpperCase() ?? "",
        })}
        badges={
          event.is_hidden ? (
            <span className="badge badge-neutral badge-sm gap-1">
              <EyeOff className="size-3" />
              {t("events.hiddenBadge", "Hidden")}
            </span>
          ) : null
        }
        description={eventDateRange(event, i18n.resolvedLanguage)}
        actions={
          <>
            <button
              type="button"
              className="btn btn-ghost btn-sm gap-1.5 rounded-full"
              onClick={() => setIsAddOpen(true)}
              disabled={isAdding}
            >
              <Plus className="size-3.5" />
              {t("events.addMedia", "Add media")}
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm gap-1.5 rounded-full"
              onClick={() => setIsShareOpen(true)}
              disabled={shareTooLarge || isSharing}
              title={
                shareTooLarge
                  ? t("events.shareTooLarge", "Events with more than 5,000 items cannot be shared.")
                  : undefined
              }
            >
              <Share2 className="size-3.5" />
              {t("common.share", "Share")}
            </button>
          </>
        }
        edit={{
          onOpen: () => setIsEditOpen(true),
          label: t("common.edit", "Edit"),
          modal: (
            <EventEditModal
              open={isEditOpen}
              event={event}
              isSaving={isPatching}
              isMerging={isMerging}
              onClose={() => setIsEditOpen(false)}
              onSave={onPatch}
              onMerge={onMerge}
            />
          ),
        }}
        stats={
          <>
            <MetaStat>
              {t("events.mediaCount", "{{count}} media").replace(
                "{{count}}",
                String(event.media_count ?? 0),
              )}
            </MetaStat>
            {(event.displayable_count ?? 0) !== (event.media_count ?? 0) && (
              <MetaStat>
                {t("events.displayableCount", "{{count}} available", {
                  count: event.displayable_count ?? 0,
                })}
              </MetaStat>
            )}
            {event.timezone && <MetaStat>{event.timezone}</MetaStat>}
          </>
        }
        footer={
          event.pending_rebuild ? (
            <div role="status" className="alert alert-info alert-soft mt-4 max-w-2xl py-2 text-sm">
              {t(
                "events.pendingRebuild",
                "Event recognition is rebuilding. This Event will update when it finishes.",
              )}
            </div>
          ) : null
        }
      />
      <EventShareModal
        open={isShareOpen}
        event={event}
        isSharing={isSharing}
        onClose={() => setIsShareOpen(false)}
        onShare={onShare}
      />
      <EventAddMediaModal
        open={isAddOpen}
        eventId={event.event_id ?? ""}
        isAdding={isAdding}
        onClose={() => setIsAddOpen(false)}
        onAdd={onAdd}
      />
    </>
  );
}

import {
  CalendarRange,
  EyeOff,
  Image,
  Merge,
  MoveRight,
  Pencil,
  Plus,
  Scissors,
  Share2,
  Trash2,
} from "lucide-react";
import { useState } from "react";
import { Navigate, useParams } from "react-router-dom";
import { AssetBrowser, AssetBrowserScope, PhotoPicker } from "@/features/assets";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { useEvent } from "../../api/useEvents";
import { eventDateRange, eventTitle } from "../../model/event";
import type { AssetsBulkActionInput } from "@/lib/assets/bulkActions";

export function EventDetailFlow() {
  const [addingMedia, setAddingMedia] = useState(false);
  const { eventId } = useParams<{ eventId: string }>();
  const { t, i18n } = useI18n();
  const eventQuery = useEvent(eventId);
  const event = eventQuery.data;
  useBreadcrumbs([
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("events.title", "Events"), to: "/collections/events" },
    { label: event ? eventTitle(event, t) : t("events.generatedTitle", "Event") },
  ]);
  if (!eventId) return <Navigate to="/collections/events" replace />;
  if (eventQuery.isPending) return <div className="skeleton m-6 h-40" />;
  if (eventQuery.isError || !event) {
    return (
      <div role="alert" className="alert alert-error alert-soft m-6">
        {t("events.detailError", "This Event could not be loaded.")}
      </div>
    );
  }
  const rename = async () => {
    const next = window.prompt(t("events.renamePrompt", "Event name"), event.title_override ?? "");
    if (next === null) return;
    if (next.trim()) await eventQuery.patch({ title_override: next.trim() });
    else await eventQuery.patch({ clear_title_override: true });
  };
  const share = async () => {
    const response = await eventQuery.share({
      title: eventTitle(event, t),
      allow_download: false,
      include_originals: false,
    });
    await navigator.clipboard.writeText(`${window.location.origin}/s/${response.token}`);
  };
  const merge = async () => {
    const other = window.prompt(t("events.mergePrompt", "Event ID to merge into this Event"));
    if (other?.trim()) await eventQuery.merge(other.trim());
  };
  const bulkActions: AssetsBulkActionInput = (context) => {
    const logical = context.selectedLogicalMedia;
    const mediaIDs = [...new Set(logical.flatMap((item) => item.media_item_ids))];
    const single = logical.length === 1 && logical[0]?.complete && mediaIDs.length === 1;
    return [
      {
        id: "event-cover",
        label: t("events.setCover", "Set as cover"),
        icon: <Image className="size-4" />,
        disabled: !single || eventQuery.isCorrecting,
        onRun: async () => {
          if (!single) return;
          await eventQuery.patch({ cover_media_item_id: mediaIDs[0] });
          context.clearSelection();
        },
      },
      {
        id: "event-split",
        label: t("events.splitBefore", "Split before selected"),
        icon: <Scissors className="size-4" />,
        disabled: !single || eventQuery.isCorrecting,
        onRun: async () => {
          if (!single) return;
          await eventQuery.split(mediaIDs[0]);
          context.clearSelection();
        },
      },
      {
        id: "event-move",
        label: t("events.moveSelected", "Move to another Event"),
        icon: <MoveRight className="size-4" />,
        disabled:
          mediaIDs.length === 0 ||
          logical.some((item) => !item.complete) ||
          eventQuery.isCorrecting,
        onRun: async () => {
          const target = window.prompt(t("events.movePrompt", "Destination Event ID"));
          if (!target?.trim()) return;
          const assetIDs = [...new Set(logical.flatMap((item) => item.representative_asset_ids))];
          await eventQuery.addAssets(assetIDs, target.trim());
          context.clearSelection();
        },
      },
      {
        id: "event-remove",
        label: t("events.removeSelected", "Remove from Event"),
        icon: <Trash2 className="size-4" />,
        tone: "danger",
        disabled:
          mediaIDs.length === 0 ||
          logical.some((item) => !item.complete) ||
          eventQuery.isCorrecting,
        requiresConfirmation: true,
        confirmationTitle: t("events.removeConfirmTitle", "Remove selected media?"),
        confirmationMessage: t(
          "events.removeConfirmMessage",
          "The media stays in your library and will be excluded from this Event.",
        ),
        onRun: async () => {
          for (const mediaID of mediaIDs) await eventQuery.remove(mediaID);
          context.clearSelection();
        },
      },
    ];
  };
  const hero = (
    <section className="card card-border bg-base-100">
      <div className="card-body gap-3">
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{eventTitle(event, t)}</h1>
            <p className="mt-1 text-sm text-base-content/60">
              {eventDateRange(event, i18n.resolvedLanguage)} ·{" "}
              {t("events.mediaCount", "{{count}} media").replace(
                "{{count}}",
                String(event.media_count),
              )}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn btn-sm" onClick={() => void rename()}>
              <Pencil className="size-4" />
              {t("common.rename", "Rename")}
            </button>
            <button
              type="button"
              className="btn btn-sm"
              disabled={eventQuery.isCorrecting}
              onClick={() => void merge()}
            >
              <Merge className="size-4" />
              {t("events.merge", "Merge")}
            </button>
            <button
              type="button"
              className="btn btn-sm"
              disabled={eventQuery.isCorrecting}
              onClick={() => setAddingMedia(true)}
            >
              <Plus className="size-4" />
              {t("events.addMedia", "Add media")}
            </button>
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => void eventQuery.patch({ is_hidden: !event.is_hidden })}
            >
              <EyeOff className="size-4" />
              {event.is_hidden ? t("events.unhide", "Unhide") : t("events.hide", "Hide")}
            </button>
            <button
              type="button"
              className="btn btn-sm"
              disabled={(event.displayable_count ?? 0) > 5000 || eventQuery.isSharing}
              onClick={() => void share()}
            >
              <Share2 className="size-4" />
              {t("common.share", "Share")}
            </button>
          </div>
        </div>
        {event.pending_rebuild && (
          <div role="status" className="alert alert-info alert-soft">
            {t("events.pendingRebuild", "Changes are waiting for an Event rebuild.")}
          </div>
        )}
      </div>
    </section>
  );
  return (
    <AssetBrowserScope
      scopeId={`event:${event.event_id}`}
      basePath={`/collections/events/${event.event_id}`}
    >
      <AssetBrowser
        title={eventTitle(event, t)}
        icon={<CalendarRange className="size-6" />}
        constraint={{ event_id: event.event_id }}
        hero={hero}
        bulkActions={bulkActions}
        viewKey={`event:${event.event_id}`}
      />
      {addingMedia && (
        <div className="fixed inset-0 z-50 bg-base-300/60 p-4 backdrop-blur-sm">
          <div className="card mx-auto h-full max-w-5xl bg-base-100">
            <div className="card-body min-h-0 p-3">
              <div className="flex items-center justify-between gap-3">
                <h2 className="card-title text-base">{t("events.addMedia", "Add media")}</h2>
                <button type="button" className="btn btn-sm" onClick={() => setAddingMedia(false)}>
                  {t("common.cancel", "Cancel")}
                </button>
              </div>
              <div className="min-h-0 flex-1 overflow-hidden rounded-box border border-base-300">
                <PhotoPicker
                  scopeId={`event-add:${event.event_id}`}
                  title={t("events.addMedia", "Add media")}
                  onSelect={(assetID) => {
                    void eventQuery.addAssets([assetID]).then(() => setAddingMedia(false));
                  }}
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </AssetBrowserScope>
  );
}

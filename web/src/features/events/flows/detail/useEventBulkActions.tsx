import { useCallback, useState } from "react";
import { Image, MoveRight, Scissors, Trash2 } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { CreateShareLinkModal, createShareSelectedBulkAction } from "@/features/share";
import type { AssetsBulkActionContext } from "@/lib/assets/bulkActions";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import type { EventDetail, EventPatch } from "../../model/event";
import EventMoveModal from "./components/EventMoveModal";

type EventCorrectionOperations = {
  patch: (patch: EventPatch) => Promise<unknown>;
  split: (mediaItemId: string) => Promise<unknown>;
  addAssets: (assetIds: string[], targetEventId?: string) => Promise<unknown>;
  remove: (mediaItemId: string) => Promise<unknown>;
  isCorrecting: boolean;
  isAdding: boolean;
};

type MoveRequest = {
  context: AssetsBulkActionContext;
  assetIds: string[];
};

/** Event-specific gallery actions and their supporting modal state. */
export function useEventBulkActions(event: EventDetail, operations: EventCorrectionOperations) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [moveRequest, setMoveRequest] = useState<MoveRequest | null>(null);
  const [shareAssetIds, setShareAssetIds] = useState<string[] | null>(null);

  const bulkActions = useCallback(
    (context: AssetsBulkActionContext) => {
      const logical = context.selectedLogicalMedia;
      const mediaIds = [...new Set(logical.flatMap((item) => item.media_item_ids))];
      const assetIds = [...new Set(logical.flatMap((item) => item.representative_asset_ids))];
      const complete = logical.every((item) => item.complete);
      const single = logical.length === 1 && complete && mediaIds.length === 1;
      return [
        createShareSelectedBulkAction(
          t("assets.assetsPageHeader.bulkActions.share.label", "Share"),
          (selectedAssetIds) => setShareAssetIds(selectedAssetIds),
        ),
        {
          id: "event-cover",
          label: t("events.setCover", "Set as cover"),
          icon: <Image className="size-4" />,
          disabled: !single || operations.isCorrecting,
          onRun: async () => {
            if (!single) return;
            try {
              await operations.patch({ cover_media_item_id: mediaIds[0] });
              context.clearSelection();
              showMessage("success", t("events.coverSuccess", "Event cover updated"));
            } catch (error) {
              showMessage(
                "error",
                t("events.coverError", "Failed to update the Event cover: {{message}}", {
                  message: localizeAPIProblem(error, t, t("home.errors.unknown")),
                }),
              );
              throw error;
            }
          },
        },
        {
          id: "event-split",
          label: t("events.splitBefore", "Split before selected"),
          icon: <Scissors className="size-4" />,
          disabled: !single || operations.isCorrecting,
          requiresConfirmation: true,
          confirmationTitle: t("events.splitConfirmTitle", "Split this Event?"),
          confirmationMessage: t(
            "events.splitConfirmMessage",
            "A new Event will begin at the selected item. The original files remain unchanged.",
          ),
          onRun: async () => {
            if (!single) return;
            try {
              await operations.split(mediaIds[0]);
              context.clearSelection();
              showMessage("success", t("events.splitSuccess", "Event split completed"));
            } catch (error) {
              showMessage(
                "error",
                t("events.splitError", "Failed to split Event: {{message}}", {
                  message: localizeAPIProblem(error, t, t("home.errors.unknown")),
                }),
              );
              throw error;
            }
          },
        },
        {
          id: "event-move",
          label: t("events.moveSelected", "Move to another Event"),
          icon: <MoveRight className="size-4" />,
          disabled: mediaIds.length === 0 || !complete || operations.isCorrecting,
          onRun: () => setMoveRequest({ context, assetIds }),
        },
        {
          id: "event-remove",
          label: t("events.removeSelected", "Remove from Event"),
          icon: <Trash2 className="size-4" />,
          tone: "danger" as const,
          disabled:
            mediaIds.length === 0 ||
            !complete ||
            mediaIds.length >= (event.media_count ?? 0) ||
            operations.isCorrecting,
          requiresConfirmation: true,
          confirmationTitle: t("events.removeConfirmTitle", "Remove selected media?"),
          confirmationMessage: t(
            "events.removeConfirmMessage",
            "The media stays in its Repository and will be excluded from this Event.",
          ),
          onRun: async () => {
            try {
              for (const mediaId of mediaIds) await operations.remove(mediaId);
              context.clearSelection();
              showMessage(
                "success",
                t("events.removeSuccess", "Removed {{count}} items from the Event", {
                  count: mediaIds.length,
                }),
              );
            } catch (error) {
              showMessage(
                "error",
                t("events.removeError", "Failed to remove media: {{message}}", {
                  message: localizeAPIProblem(error, t, t("home.errors.unknown")),
                }),
              );
              throw error;
            }
          },
        },
      ];
    },
    [event.media_count, operations, showMessage, t],
  );

  const dialogs = (
    <>
      <EventMoveModal
        open={moveRequest !== null}
        currentEventId={event.event_id ?? ""}
        selectedCount={moveRequest?.context.selectedItemCount ?? 0}
        isMoving={operations.isAdding}
        onClose={() => setMoveRequest(null)}
        onMove={async (targetEventId) => {
          if (!moveRequest) return;
          await operations.addAssets(moveRequest.assetIds, targetEventId);
          moveRequest.context.clearSelection();
        }}
      />
      <CreateShareLinkModal
        open={shareAssetIds !== null}
        onClose={() => setShareAssetIds(null)}
        sourceKind="asset_snapshot"
        assetIds={shareAssetIds ?? undefined}
      />
    </>
  );

  return { bulkActions, dialogs };
}

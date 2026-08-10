import { useEffect, useState } from "react";
import { MoveRight } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import EventPicker from "./EventPicker";

type EventMoveModalProps = {
  open: boolean;
  currentEventId: string;
  selectedCount: number;
  isMoving: boolean;
  onClose: () => void;
  onMove: (targetEventId: string) => Promise<unknown>;
};

/** Destination picker and confirmation for moving selected logical media. */
export default function EventMoveModal({
  open,
  currentEventId,
  selectedCount,
  isMoving,
  onClose,
  onMove,
}: EventMoveModalProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [selectedIds, setSelectedIds] = useState<string[]>([]);

  useEffect(() => {
    if (open) setSelectedIds([]);
  }, [open]);

  const handleMove = async () => {
    const targetId = selectedIds[0];
    if (!targetId || isMoving) return;
    try {
      await onMove(targetId);
      showMessage(
        "success",
        t("events.moveSuccess", "Moved {{count}} items to the selected Event", {
          count: selectedCount,
        }),
      );
      onClose();
    } catch (error) {
      showMessage(
        "error",
        t("events.moveError", "Failed to move media: {{message}}", {
          message: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="md"
      dismissable={!isMoving}
      icon={<MoveRight size={20} />}
      title={t("events.moveTitle", "Move to another Event")}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={isMoving}>
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => void handleMove()}
            disabled={selectedIds.length !== 1 || isMoving}
          >
            {isMoving && <span className="loading loading-spinner loading-sm" />}
            {t("events.moveConfirm", "Move media")}
          </button>
        </>
      }
    >
      <div className="space-y-4 p-5">
        <p className="text-sm leading-6 text-base-content/65">
          {t(
            "events.moveDescription",
            "Choose the destination for {{count}} selected items. The files remain in their Repositories.",
            { count: selectedCount },
          )}
        </p>
        <EventPicker
          excludeIds={[currentEventId]}
          selectedIds={selectedIds}
          onChange={setSelectedIds}
        />
      </div>
    </Modal>
  );
}

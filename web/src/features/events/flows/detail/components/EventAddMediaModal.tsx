import { Plus } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { PhotoPicker } from "@/features/assets";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";

type EventAddMediaModalProps = {
  open: boolean;
  eventId: string;
  isAdding: boolean;
  onClose: () => void;
  onAdd: (assetIds: string[]) => Promise<unknown>;
};

/** Multi-select media picker for explicitly adding members to an Event. */
export default function EventAddMediaModal({
  open,
  eventId,
  isAdding,
  onClose,
  onAdd,
}: EventAddMediaModalProps) {
  const { t } = useI18n();
  const showMessage = useMessage();

  const handleAdd = async (assetIds: string[]) => {
    if (assetIds.length === 0) return;
    try {
      await onAdd(assetIds);
      showMessage(
        "success",
        t("events.addMediaSuccess", "Added {{count}} items to the Event", {
          count: assetIds.length,
        }),
      );
      onClose();
    } catch (error) {
      showMessage(
        "error",
        t("events.addMediaError", "Failed to add media: {{message}}", {
          message: localizeAPIProblem(error, t, t("home.errors.unknown")),
        }),
      );
      throw error;
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="xl"
      dismissable={!isAdding}
      icon={<Plus size={20} />}
      title={t("events.addMedia", "Add media")}
      className="h-[min(820px,90vh)]"
      bodyClassName="overflow-hidden"
      footer={
        <button type="button" className="btn btn-ghost" onClick={onClose} disabled={isAdding}>
          {t("common.close", "Close")}
        </button>
      }
    >
      <PhotoPicker
        scopeId={`event-add:${eventId}`}
        title={t("events.addMediaPickerTitle", "Choose media")}
        selectionMode="multiple"
        typeFilter={null}
        confirmLabel={t("events.addSelectedMedia", "Add selected")}
        isConfirming={isAdding}
        onConfirm={handleAdd}
      />
    </Modal>
  );
}

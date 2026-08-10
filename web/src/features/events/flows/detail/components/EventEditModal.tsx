import { useEffect, useState } from "react";
import { CalendarRange, EyeOff, Merge, Pencil } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import type { EventDetail, EventPatch } from "../../../model/event";
import { eventTitle } from "../../../model/event";
import EventPicker from "./EventPicker";

type EditTab = "info" | "merge";

type EventEditModalProps = {
  open: boolean;
  event: EventDetail;
  isSaving: boolean;
  isMerging: boolean;
  onClose: () => void;
  onSave: (patch: EventPatch) => Promise<unknown>;
  onMerge: (eventIds: string[]) => Promise<unknown>;
};

/** Metadata and destructive identity corrections for one owner-scoped Event. */
export default function EventEditModal({
  open,
  event,
  isSaving,
  isMerging,
  onClose,
  onSave,
  onMerge,
}: EventEditModalProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [activeTab, setActiveTab] = useState<EditTab>("info");
  const [name, setName] = useState(event.title_override ?? "");
  const [isHidden, setIsHidden] = useState(Boolean(event.is_hidden));
  const [mergeIds, setMergeIds] = useState<string[]>([]);
  const [confirmingMerge, setConfirmingMerge] = useState(false);

  useEffect(() => {
    if (!open) return;
    setActiveTab("info");
    setName(event.title_override ?? "");
    setIsHidden(Boolean(event.is_hidden));
    setMergeIds([]);
    setConfirmingMerge(false);
  }, [event.event_id, event.is_hidden, event.title_override, open]);

  const trimmedName = name.trim();
  const nameChanged = trimmedName !== (event.title_override ?? "");
  const hiddenChanged = isHidden !== Boolean(event.is_hidden);
  const canSave = (nameChanged || hiddenChanged) && !isSaving;

  const handleSave = async (formEvent: React.FormEvent) => {
    formEvent.preventDefault();
    if (!canSave) return;
    const patch: EventPatch = {};
    if (nameChanged) {
      if (trimmedName) patch.title_override = trimmedName;
      else patch.clear_title_override = true;
    }
    if (hiddenChanged) patch.is_hidden = isHidden;
    try {
      await onSave(patch);
      showMessage("success", t("events.edit.success", "Event updated"));
      onClose();
    } catch (error) {
      showMessage(
        "error",
        t("events.edit.error", "Failed to update Event: {{message}}", {
          message: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  const handleMerge = async () => {
    if (mergeIds.length === 0 || isMerging) return;
    if (!confirmingMerge) {
      setConfirmingMerge(true);
      return;
    }
    try {
      await onMerge(mergeIds);
      showMessage(
        "success",
        t("events.mergeSuccess", "Merged {{count}} Events", { count: mergeIds.length + 1 }),
      );
      onClose();
    } catch (error) {
      showMessage(
        "error",
        t("events.mergeError", "Failed to merge Events: {{message}}", {
          message: error instanceof Error ? error.message : String(error),
        }),
      );
      setConfirmingMerge(false);
    }
  };

  const footer = (
    <button
      type="button"
      className="btn btn-ghost"
      onClick={onClose}
      disabled={isSaving || isMerging}
    >
      {t("common.close", "Close")}
    </button>
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="md"
      dismissable={!isSaving && !isMerging}
      icon={<CalendarRange size={20} />}
      title={t("events.edit.title", "Edit Event")}
      footer={footer}
    >
      <div className="flex min-h-0 flex-col">
        <div role="tablist" className="tabs tabs-border flex-shrink-0 px-5 pt-3">
          <button
            type="button"
            role="tab"
            className={`tab gap-2 ${activeTab === "info" ? "tab-active" : ""}`}
            onClick={() => setActiveTab("info")}
          >
            <Pencil className="size-4" />
            {t("events.edit.infoTab", "Info")}
          </button>
          <button
            type="button"
            role="tab"
            className={`tab gap-2 ${activeTab === "merge" ? "tab-active" : ""}`}
            onClick={() => setActiveTab("merge")}
          >
            <Merge className="size-4" />
            {t("events.merge", "Merge")}
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {activeTab === "info" ? (
            <form onSubmit={handleSave} className="space-y-5">
              <fieldset className="fieldset w-full py-0">
                <legend className="fieldset-legend pb-1 text-xs font-semibold tracking-wide text-base-content/55">
                  {t("events.edit.nameLabel", "Name")}
                </legend>
                <input
                  value={name}
                  onChange={(formEvent) => setName(formEvent.target.value)}
                  placeholder={eventTitle(event, t)}
                  className="input input-bordered w-full"
                />
                <p className="label text-xs leading-5 text-base-content/55">
                  {t(
                    "events.edit.nameHint",
                    "Leave the name empty to use the automatic Event title.",
                  )}
                </p>
              </fieldset>

              <div className="flex flex-wrap items-center justify-between gap-4 border-t border-base-200 pt-4">
                <div className="min-w-0 space-y-1">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <EyeOff className="size-4 text-base-content/55" />
                    {t("events.edit.hiddenLabel", "Hidden")}
                  </div>
                  <p className="max-w-lg text-xs leading-5 text-base-content/60">
                    {t(
                      "events.edit.hiddenHint",
                      "Hide this Event from Event lists without changing its media or corrections.",
                    )}
                  </p>
                </div>
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={isHidden}
                  onChange={(formEvent) => setIsHidden(formEvent.target.checked)}
                  aria-label={t("events.edit.hiddenLabel", "Hidden")}
                />
              </div>

              <div className="flex justify-end border-t border-base-200 pt-4">
                <button type="submit" className="btn btn-primary btn-sm" disabled={!canSave}>
                  {isSaving && <span className="loading loading-spinner loading-xs" />}
                  {t("common.save", "Save")}
                </button>
              </div>
            </form>
          ) : (
            <div className="space-y-4">
              <div>
                <h4 className="text-sm font-semibold">{t("events.mergeTitle", "Merge Events")}</h4>
                <p className="mt-1 text-xs leading-5 text-base-content/65">
                  {t(
                    "events.mergeDescription",
                    "Selected Events will be merged into {{name}}. Media stays in its Repositories and this Event remains the destination.",
                    { name: eventTitle(event, t) },
                  )}
                </p>
              </div>

              <EventPicker
                excludeIds={event.event_id ? [event.event_id] : []}
                selectedIds={mergeIds}
                onChange={(ids) => {
                  setMergeIds(ids);
                  setConfirmingMerge(false);
                }}
                multiSelect
              />

              {confirmingMerge && (
                <div role="alert" className="alert alert-warning alert-soft text-sm">
                  {t(
                    "events.mergeConfirmMessage",
                    "This will move the selected Events into the current Event and redirect their old links.",
                  )}
                </div>
              )}

              <div className="flex items-center justify-between gap-3 border-t border-base-200 pt-4">
                <span className="text-xs text-base-content/55">
                  {t("events.mergeSelectedCount", "{{count}} selected", {
                    count: mergeIds.length,
                  })}
                </span>
                <div className="flex gap-2">
                  {confirmingMerge && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setConfirmingMerge(false)}
                      disabled={isMerging}
                    >
                      {t("common.cancel")}
                    </button>
                  )}
                  <button
                    type="button"
                    className={`btn btn-sm ${confirmingMerge ? "btn-warning" : "btn-primary"}`}
                    onClick={() => void handleMerge()}
                    disabled={mergeIds.length === 0 || isMerging}
                  >
                    {isMerging && <span className="loading loading-spinner loading-xs" />}
                    {confirmingMerge
                      ? t("events.mergeConfirm", "Confirm merge")
                      : t("events.mergeReview", "Review merge")}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}

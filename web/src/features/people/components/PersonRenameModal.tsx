import React, { useEffect, useState } from "react";
import { ScanFace, UserRound, Users } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import Modal from "@/components/Modal";
import PersonFacesPanel from "./PersonFacesPanel";
import PersonPicker from "./PersonPicker";
import { useMergePeople, useSetPersonHidden } from "../hooks/usePeople";
import type { PersonDetail } from "../people.types";

interface PersonRenameModalProps {
  open: boolean;
  /** Person being edited; optional while the detail query is still loading. */
  person?: PersonDetail;
  /** Current name, used to seed the field. */
  currentName: string;
  isSaving: boolean;
  onClose: () => void;
  /** Commit the new name; resolve to close. */
  onSubmit: (name: string) => Promise<void> | void;
}

type EditTab = "info" | "faces" | "merge";

/**
 * Person editing surface. It keeps identity management out of the photo gallery:
 * basic metadata, face-level corrections and person merges live in tabs here.
 */
export function PersonRenameModal({
  open,
  person,
  currentName,
  isSaving,
  onClose,
  onSubmit,
}: PersonRenameModalProps): React.ReactNode {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [name, setName] = useState(currentName);
  const [isHidden, setIsHidden] = useState(Boolean(person?.is_hidden));
  const [activeTab, setActiveTab] = useState<EditTab>("info");
  const [sourceIds, setSourceIds] = useState<number[]>([]);
  const { setPersonHidden, isUpdatingHidden } = useSetPersonHidden();
  const { mergePeople, isMerging } = useMergePeople();

  useEffect(() => {
    if (open) {
      setName(currentName);
      setIsHidden(Boolean(person?.is_hidden));
      setActiveTab("info");
      setSourceIds([]);
    }
  }, [open, currentName, person?.is_hidden]);

  const trimmed = name.trim();
  const canSave = trimmed.length > 0 && trimmed !== currentName && !isSaving;
  const personId = person?.person_id ?? 0;
  const canMerge = personId > 0 && sourceIds.length > 0 && !isMerging;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSave) return;
    await onSubmit(trimmed);
  };

  const handleHiddenChange = async () => {
    if (!personId || isUpdatingHidden) return;
    const nextHidden = !isHidden;
    setIsHidden(nextHidden);
    try {
      await setPersonHidden(personId, nextHidden);
      showMessage(
        "success",
        nextHidden
          ? t("people.hidden.hiddenToast", "Person hidden")
          : t("people.hidden.unhiddenToast", "Person restored"),
      );
    } catch (err) {
      showMessage(
        "error",
        t("people.hidden.error", "Failed to update person: {{message}}", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
      setIsHidden(!nextHidden);
    }
  };

  const handleMerge = async () => {
    if (!canMerge) return;
    try {
      await mergePeople(personId, sourceIds);
      setSourceIds([]);
      showMessage("success", t("people.merge.success", "People merged"));
    } catch (err) {
      showMessage(
        "error",
        t("people.merge.error", "Failed to merge people: {{message}}", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
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

  const tabs: Array<{ id: EditTab; label: string; icon: React.ReactNode }> = [
    {
      id: "info",
      label: t("people.edit.tabs.info", "Info"),
      icon: <UserRound className="size-4" />,
    },
    {
      id: "faces",
      label: t("people.edit.tabs.faces", "Faces"),
      icon: <ScanFace className="size-4" />,
    },
    {
      id: "merge",
      label: t("people.edit.tabs.merge", "Merge"),
      icon: <Users className="size-4" />,
    },
  ];
  const isWorkspaceTab = activeTab === "faces";

  return (
    <Modal
      open={open}
      onClose={onClose}
      size={isWorkspaceTab ? "lg" : "md"}
      dismissable={!isSaving && !isMerging}
      icon={<UserRound size={20} />}
      title={t("people.edit.title", "Edit person")}
      footer={footer}
      className={isWorkspaceTab ? "h-[min(720px,88vh)]" : ""}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div role="tablist" className="tabs tabs-border flex-shrink-0 px-5 pt-3">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              className={`tab gap-2 ${activeTab === tab.id ? "tab-active" : ""}`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {activeTab === "info" && (
            <div className="max-w-2xl space-y-4">
              <form id="person-rename-form" onSubmit={handleSubmit} className="space-y-2">
                <fieldset className="fieldset w-full py-0">
                  <legend className="fieldset-legend pb-1 text-xs font-semibold uppercase tracking-wide text-base-content/55">
                    {t("people.details.nameLabel", "Name")}
                  </legend>
                  <div className="flex flex-col gap-2 sm:flex-row">
                    {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
                    <input
                      autoFocus
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder={t("people.details.renamePlaceholder", "Name this person")}
                      className="input input-bordered w-full sm:flex-1"
                    />
                    <button type="submit" className="btn btn-primary sm:w-28" disabled={!canSave}>
                      {isSaving && <span className="loading loading-spinner loading-sm" />}
                      {t("people.details.save", "Save")}
                    </button>
                  </div>
                </fieldset>
              </form>

              <div className="flex flex-wrap items-center justify-between gap-4 border-t border-base-200 pt-4">
                <div className="min-w-0 space-y-0.5">
                  <div className="text-sm font-medium">
                    {t("people.hidden.settingTitle", "Hidden")}
                  </div>
                  <p className="max-w-lg text-xs leading-5 text-base-content/60">
                    {t(
                      "people.hidden.settingDescription",
                      "Hide this person from the default people grid without changing photos or face assignments.",
                    )}
                  </p>
                </div>
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={isHidden}
                  onChange={handleHiddenChange}
                  disabled={!personId || isUpdatingHidden}
                  aria-label={t("people.hidden.settingTitle", "Hidden")}
                />
              </div>
            </div>
          )}

          {activeTab === "faces" && personId > 0 && <PersonFacesPanel personId={personId} />}

          {activeTab === "merge" && (
            <div className="max-w-2xl space-y-4">
              <div>
                <h4 className="text-sm font-semibold">{t("people.merge.title", "Merge people")}</h4>
                <p className="text-xs leading-5 text-base-content/65">
                  {t(
                    "people.merge.description",
                    "The selected people will be merged into {{name}}. Their photos stay in the library and people filters may change.",
                    { name: person?.name || t("people.unnamed", "Unnamed") },
                  )}
                </p>
              </div>
              <PersonPicker
                excludeIds={personId ? [personId] : []}
                selectedIds={sourceIds}
                onChange={setSourceIds}
                multiSelect
              />
              <div className="flex items-center justify-between gap-3 border-t border-base-200 pt-3">
                <span className="text-xs text-base-content/55">
                  {t("people.merge.selectedCount", "{{count}} selected", {
                    count: sourceIds.length,
                  })}
                </span>
                <button
                  type="button"
                  className="btn btn-primary btn-sm"
                  onClick={handleMerge}
                  disabled={!canMerge}
                >
                  {isMerging && <span className="loading loading-spinner loading-sm" />}
                  {t("people.merge.confirm", "Merge")}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}

export default PersonRenameModal;

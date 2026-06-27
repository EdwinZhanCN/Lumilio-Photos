import React, { useEffect, useState } from "react";
import { UserRound } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import Modal from "@/components/Modal";

interface PersonRenameModalProps {
  open: boolean;
  /** Current name, used to seed the field. */
  currentName: string;
  isSaving: boolean;
  onClose: () => void;
  /** Commit the new name; resolve to close. */
  onSubmit: (name: string) => Promise<void> | void;
}

/**
 * Rename a person through the shared modal shell so person editing matches the
 * album editing mental model (everything edits via a modal, not inline).
 */
export function PersonRenameModal({
  open,
  currentName,
  isSaving,
  onClose,
  onSubmit,
}: PersonRenameModalProps): React.ReactNode {
  const { t } = useI18n();
  const [name, setName] = useState(currentName);

  useEffect(() => {
    if (open) setName(currentName);
  }, [open, currentName]);

  const trimmed = name.trim();
  const canSave = trimmed.length > 0 && trimmed !== currentName && !isSaving;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSave) return;
    await onSubmit(trimmed);
    onClose();
  };

  const footer = (
    <>
      <button type="button" className="btn btn-ghost" onClick={onClose} disabled={isSaving}>
        {t("common.cancel")}
      </button>
      <button type="submit" form="person-rename-form" className="btn btn-primary" disabled={!canSave}>
        {isSaving && <span className="loading loading-spinner loading-sm" />}
        {t("people.details.save")}
      </button>
    </>
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="sm"
      dismissable={!isSaving}
      icon={<UserRound size={20} />}
      title={t("people.details.renameTitle", "Rename person")}
      footer={footer}
    >
      <form id="person-rename-form" onSubmit={handleSubmit} className="p-6">
        <fieldset className="fieldset w-full">
          <legend className="fieldset-legend font-semibold">
            {t("people.details.nameLabel", "Name")}
          </legend>
          {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("people.details.renamePlaceholder")}
            className="input input-bordered w-full"
          />
        </fieldset>
      </form>
    </Modal>
  );
}

export default PersonRenameModal;

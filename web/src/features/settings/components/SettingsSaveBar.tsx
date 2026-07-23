/**
 * SettingsSaveBar — a restrained sticky footer for manual-save tabs.
 *
 * Instant prefs apply on change; DB-backed drafts collect edits and commit
 * on Save. Rather than a badge per group, a single bar slides up at the
 * bottom of the centered content column while the draft is dirty (or
 * saving / just saved / errored), carrying the status and the
 * Reset / Save actions. One per tab — each tab has at most one draft.
 */
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, Check, Loader2 } from "lucide-react";

interface SettingsSaveBarProps {
  isDirty?: boolean;
  isSaving?: boolean;
  justSaved?: boolean;
  error?: unknown;
  canSave?: boolean;
  onSave: () => void;
  onReset: () => void;
  saveLabel?: string;
  resetLabel?: string;
  /** Extra action rendered to the left of Reset (e.g. "Validate"). */
  extraAction?: ReactNode;
}

function errorMessage(error: unknown, fallback: string): string | null {
  if (!error) return null;
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return fallback;
}

export function SettingsSaveBar({
  isDirty,
  isSaving,
  justSaved,
  error,
  canSave,
  onSave,
  onReset,
  saveLabel,
  resetLabel,
  extraAction,
}: SettingsSaveBarProps) {
  const { t } = useTranslation();
  const err = errorMessage(error, t("settings.section.saveFailed", { defaultValue: "Save failed" }));
  const visible = Boolean(err) || isSaving || justSaved || isDirty;

  return (
    <div
      className={`pointer-events-none sticky bottom-4 z-sticky mt-2 transition-all duration-200 ${
        visible ? "translate-y-0 opacity-100" : "translate-y-4 opacity-0"
      }`}
      aria-hidden={!visible}
    >
      <div className="pointer-events-auto flex flex-col gap-2 rounded-2xl border border-base-300 bg-base-100/95 px-4 py-2.5 shadow-lg backdrop-blur sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm font-medium">
          {err ? (
            <span className="flex items-center gap-2 text-error">
              <AlertCircle size={16} /> {err}
            </span>
          ) : isSaving ? (
            <span className="flex items-center gap-2 text-base-content/70">
              <Loader2 size={16} className="animate-spin" />
              {t("settings.section.saving", { defaultValue: "Saving changes..." })}
            </span>
          ) : justSaved ? (
            <span className="flex items-center gap-2 text-success">
              <Check size={16} />
              {t("settings.section.saved", { defaultValue: "Changes saved" })}
            </span>
          ) : (
            <span className="flex items-center gap-2 text-base-content/70">
              <AlertCircle size={16} />
              {t("settings.section.unsavedCareful", {
                defaultValue: "Careful, you have unsaved changes.",
              })}
            </span>
          )}
        </div>
        <div className="flex items-center justify-end gap-2">
          {extraAction}
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onReset}
            disabled={!isDirty || isSaving}
          >
            {resetLabel ?? t("settings.section.reset", { defaultValue: "Reset" })}
          </button>
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={onSave}
            disabled={!canSave}
          >
            {saveLabel ?? t("settings.section.save", { defaultValue: "Save" })}
          </button>
        </div>
      </div>
    </div>
  );
}

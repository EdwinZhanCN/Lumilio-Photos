/**
 * SettingsSection renders one settings group with the interaction model that
 * matches its state origin, so a single tab can freely mix the three:
 *
 *   - "mutable"  (DB): a Save/Reset footer driven by useDraftSettings
 *                      (dirty / saving / saved / error).
 *   - "instant"  (localStorage): no footer; changes apply immediately.
 *   - "readonly" (DB read-only): no controls; display only.
 *
 * The section is presentational — callers own the fields and wire mutable
 * sections to useDraftSettings.
 */
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, Check, Loader2 } from "lucide-react";

type SettingsSectionVariant = "mutable" | "instant" | "readonly";

interface SettingsSectionProps {
  title: string;
  description?: string;
  variant: SettingsSectionVariant;
  children: ReactNode;

  // Mutable-only props (driven by useDraftSettings).
  isDirty?: boolean;
  isSaving?: boolean;
  saveError?: unknown;
  justSaved?: boolean;
  canSave?: boolean;
  onSave?: () => void;
  onReset?: () => void;
}

function errorMessage(error: unknown): string | null {
  if (!error) return null;
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "Save failed";
}

export function SettingsSection({
  title,
  description,
  variant,
  children,
  isDirty,
  isSaving,
  saveError,
  justSaved,
  canSave,
  onSave,
  onReset,
}: SettingsSectionProps) {
  const { t } = useTranslation();
  const err = errorMessage(saveError);

  return (
    <section className="rounded-lg border border-base-300 bg-base-100">
      <header className="flex items-start justify-between gap-4 border-b border-base-200 px-4 py-3">
        <div>
          <h3 className="text-sm font-semibold">{title}</h3>
          {description && (
            <p className="mt-0.5 text-xs text-base-content/60">{description}</p>
          )}
        </div>
        {variant === "instant" && (
          <span className="text-xs text-base-content/50">
            {t("settings.section.instant", { defaultValue: "Auto-saved" })}
          </span>
        )}
        {variant === "readonly" && (
          <span className="text-xs text-base-content/50">
            {t("settings.section.readonly", { defaultValue: "Read-only" })}
          </span>
        )}
      </header>

      <div className="px-4 py-3">{children}</div>

      {variant === "mutable" && (
        <footer className="flex items-center justify-between gap-3 border-t border-base-200 px-4 py-3">
          <div className="text-xs">
            {err ? (
              <span className="flex items-center gap-1 text-error">
                <AlertCircle size={14} /> {err}
              </span>
            ) : isSaving ? (
              <span className="flex items-center gap-1 text-base-content/60">
                <Loader2 size={14} className="animate-spin" />
                {t("settings.section.saving", { defaultValue: "Saving…" })}
              </span>
            ) : justSaved ? (
              <span className="flex items-center gap-1 text-success">
                <Check size={14} />
                {t("settings.section.saved", { defaultValue: "Saved" })}
              </span>
            ) : isDirty ? (
              <span className="text-warning">
                {t("settings.section.unsaved", {
                  defaultValue: "Unsaved changes",
                })}
              </span>
            ) : (
              <span className="text-base-content/40">
                {t("settings.section.upToDate", { defaultValue: "Up to date" })}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={onReset}
              disabled={!isDirty || isSaving}
            >
              {t("settings.section.reset", { defaultValue: "Reset" })}
            </button>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={onSave}
              disabled={!canSave}
            >
              {t("settings.section.save", { defaultValue: "Save" })}
            </button>
          </div>
        </footer>
      )}
    </section>
  );
}

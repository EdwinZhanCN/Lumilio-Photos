import React from "react";
import { Frame, Loader2, Wand2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { BorderPanel, isExifBorderMode, normalizeParams } from "@/features/studio/tools/border";
import type { BorderExifSummary } from "@/features/studio/tools/border/BorderPanel";
import { SectionHeader } from "./SectionHeader";

type BorderToolSectionProps = {
  open: boolean;
  onToggle: () => void;
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  onApply: () => void;
  onClear: () => void;
  isApplying: boolean;
  hasResult: boolean;
  disabled?: boolean;
  exifSummary?: BorderExifSummary;
};

/**
 * Inline "Tools" group hosting the Border tool. Unlike the non-destructive
 * develop sliders, Border bakes new pixels: applying it composes on top of the
 * current developed image and shows the result in the viewport until cleared.
 */
export function BorderToolSection({
  open,
  onToggle,
  value,
  onChange,
  onApply,
  onClear,
  isApplying,
  hasResult,
  disabled = false,
  exifSummary,
}: BorderToolSectionProps): React.JSX.Element {
  const { t } = useI18n();

  // EXIF-driven modes can't be applied without sufficient EXIF on the asset.
  const mode = normalizeParams(value).mode;
  const exifBlocked = isExifBorderMode(mode) && !(exifSummary?.available ?? false);
  const applyDisabled = disabled || isApplying || exifBlocked;

  return (
    <div className="border-b border-base-300 last:border-0">
      <SectionHeader
        icon={Frame}
        title={t("studio.tools.title", { defaultValue: "Tools" })}
        open={open}
        modified={hasResult}
        onToggle={onToggle}
      />
      {open && (
        <div className="space-y-3 pb-3.5 pl-1 pr-1 pt-1">
          <p className="text-[11px] text-base-content/45">
            {t("studio.tools.border.hint", {
              defaultValue:
                "Border bakes new pixels on top of your current edit. Apply to preview, then export.",
            })}
          </p>

          <BorderPanel
            value={value}
            onChange={onChange}
            disabled={disabled || isApplying}
            exifSummary={exifSummary}
          />

          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-primary btn-sm flex-1 gap-1.5"
              onClick={onApply}
              disabled={applyDisabled}
            >
              {isApplying ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Wand2 className="h-4 w-4" />
              )}
              {t("studio.tools.border.apply", { defaultValue: "Apply border" })}
            </button>
            {hasResult && (
              <button
                type="button"
                className="btn btn-ghost btn-sm gap-1 text-base-content/70"
                onClick={onClear}
                disabled={isApplying}
              >
                <X className="h-4 w-4" />
                {t("studio.tools.border.clear", { defaultValue: "Clear" })}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

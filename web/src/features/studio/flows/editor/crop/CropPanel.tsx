import React from "react";
import { RotateCcw } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { ASPECT_PRESETS } from "../../../modules/crop/cropMath";

type CropPanelProps = {
  aspectKey: string;
  onAspectChange: (key: string) => void;
  onReset: () => void;
  disabled?: boolean;
};

/**
 * Crop controls: aspect-ratio presets and reset. The crop box itself is dragged
 * on the photo via {@link CropOverlay}; this panel only picks the constraint.
 */
export function CropPanel({
  aspectKey,
  onAspectChange,
  onReset,
  disabled = false,
}: CropPanelProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <div className="py-3">
      <div className="mb-2 text-xs font-medium text-base-content/60">
        {t("studio.crop.aspect", { defaultValue: "Aspect ratio" })}
      </div>
      <div className="grid grid-cols-3 gap-1.5">
        {ASPECT_PRESETS.map((preset) => (
          <button
            key={preset.key}
            type="button"
            disabled={disabled}
            onClick={() => onAspectChange(preset.key)}
            aria-pressed={aspectKey === preset.key}
            className={`btn btn-sm border-base-300 text-[12px] ${
              aspectKey === preset.key
                ? "btn-active border-primary/50 text-primary"
                : "bg-base-100 text-base-content/70 hover:border-base-content/25"
            }`}
          >
            {preset.label}
          </button>
        ))}
      </div>

      <button
        type="button"
        disabled={disabled}
        onClick={onReset}
        className="btn btn-ghost btn-sm mt-3 w-full gap-1.5 text-base-content/70"
      >
        <RotateCcw size={14} />
        {t("studio.crop.reset", { defaultValue: "Reset crop" })}
      </button>

      <p className="mt-3 text-[11px] leading-relaxed text-base-content/40">
        {t("studio.crop.hint", {
          defaultValue: "Drag the handles on the photo to set the crop.",
        })}
      </p>
    </div>
  );
}

import React from "react";
import { Info, Sparkles, TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { DEFAULT_PARAMS, type BorderParams, normalizeParams } from "./types";

export const defaultParams: Record<string, unknown> = DEFAULT_PARAMS;

/**
 * Read-only summary of the EXIF/brand the editor detected for the current
 * asset. EXIF-driven border modes (Frosted Info / Info Strip) consume this; the
 * user cannot edit EXIF or pick a logo — everything is auto-matched.
 */
export type BorderExifSummary = {
  available: boolean;
  cameraLabel?: string;
  brandText?: string | null;
  hasLogo: boolean;
};

export const BorderPanel: React.FC<{
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
  exifSummary?: BorderExifSummary;
}> = ({ value, onChange, disabled, exifSummary }) => {
  const p = normalizeParams(value);
  const { t } = useI18n();

  const update = (next: Partial<BorderParams>) => {
    onChange({ ...p, ...next });
  };

  // Static `t()` calls so the i18next extractor can pick up every key.
  const tabs: Array<{ mode: BorderParams["mode"]; label: string }> = [
    { mode: "COLORED", label: t("studio.tools.border.modeColored", { defaultValue: "Colored" }) },
    { mode: "FROSTED", label: t("studio.tools.border.modeFrosted", { defaultValue: "Frosted" }) },
    { mode: "VIGNETTE", label: t("studio.tools.border.modeVignette", { defaultValue: "Vignette" }) },
    {
      mode: "FROSTED_INFO",
      label: t("studio.tools.border.modeFrostedInfo", { defaultValue: "Frosted Info" }),
    },
    {
      mode: "INFO_STRIP",
      label: t("studio.tools.border.modeInfoStrip", { defaultValue: "Info Strip" }),
    },
  ];

  return (
    <div className="space-y-4">
      <div className="tabs tabs-boxed flex-wrap">
        {tabs.map((tab) => (
          <button
            key={tab.mode}
            type="button"
            className={`tab ${p.mode === tab.mode ? "tab-active" : ""}`}
            disabled={disabled}
            onClick={() => update({ mode: tab.mode })}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {p.mode === "COLORED" && (
        <div className="space-y-2">
          <label className="label">
            {t("studio.tools.border.borderWidth", {
              defaultValue: "Border Width",
            })}
            : {p.border_width}
          </label>
          <input
            className="range range-primary"
            type="range"
            min={1}
            max={100}
            value={p.border_width}
            disabled={disabled}
            onChange={(e) => update({ border_width: Number(e.target.value) })}
          />

          <label className="label">
            {t("studio.tools.border.color", { defaultValue: "Color" })}
          </label>
          <input
            className="w-full h-10 p-1"
            type="color"
            value={p.color_hex}
            disabled={disabled}
            onChange={(e) => update({ color_hex: e.target.value })}
          />
        </div>
      )}

      {(p.mode === "FROSTED" || p.mode === "FROSTED_INFO") && (
        <div className="space-y-2">
          {p.mode === "FROSTED_INFO" && (
            <ExifReadout summary={exifSummary} t={t} />
          )}

          <label className="label">
            {t("studio.tools.border.blur", { defaultValue: "Blur" })}:{" "}
            {p.blur_sigma.toFixed(1)}
          </label>
          <input
            className="range range-primary"
            type="range"
            min={1}
            max={50}
            step={0.5}
            value={p.blur_sigma}
            disabled={disabled}
            onChange={(e) => update({ blur_sigma: Number(e.target.value) })}
          />

          <label className="label">
            {t("studio.tools.border.brightness", {
              defaultValue: "Brightness",
            })}
            : {p.brightness_adjustment}
          </label>
          <input
            className="range range-primary"
            type="range"
            min={-100}
            max={100}
            value={p.brightness_adjustment}
            disabled={disabled}
            onChange={(e) =>
              update({ brightness_adjustment: Number(e.target.value) })
            }
          />

          <label className="label">
            {t("studio.tools.border.cornerRadius", {
              defaultValue: "Corner Radius",
            })}
            : {p.corner_radius}
          </label>
          <input
            className="range range-primary"
            type="range"
            min={0}
            max={100}
            value={p.corner_radius}
            disabled={disabled}
            onChange={(e) => update({ corner_radius: Number(e.target.value) })}
          />
        </div>
      )}

      {p.mode === "VIGNETTE" && (
        <div className="space-y-2">
          <label className="label">
            {t("studio.tools.border.strength", { defaultValue: "Strength" })}:{" "}
            {p.strength.toFixed(2)}
          </label>
          <input
            className="range range-primary"
            type="range"
            min={0.1}
            max={2}
            step={0.05}
            value={p.strength}
            disabled={disabled}
            onChange={(e) => update({ strength: Number(e.target.value) })}
          />
        </div>
      )}

      {p.mode === "INFO_STRIP" && (
        <div className="space-y-2">
          <ExifReadout summary={exifSummary} t={t} />
        </div>
      )}
    </div>
  );
};

/**
 * Auto-filled EXIF / brand readout for the EXIF-driven modes. Purely
 * informational — there are no editable EXIF or logo controls by design.
 */
const ExifReadout: React.FC<{
  summary?: BorderExifSummary;
  t: ReturnType<typeof useI18n>["t"];
}> = ({ summary, t }) => {
  if (!summary || !summary.available) {
    return (
      <div className="flex items-start gap-2 rounded-md bg-warning/10 px-2.5 py-2 text-[11px] text-warning">
        <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span>
          {t("studio.tools.border.exifMissing", {
            defaultValue:
              "This style needs camera EXIF (model + at least one of focal length, aperture, shutter, ISO). It's unavailable for this photo.",
          })}
        </span>
      </div>
    );
  }

  const brandLabel = summary.brandText
    ? summary.hasLogo
      ? t("studio.tools.border.brandLogo", {
          defaultValue: "{{brand}} (logo)",
          brand: summary.brandText,
        })
      : t("studio.tools.border.brandText", {
          defaultValue: "{{brand}} (text)",
          brand: summary.brandText,
        })
    : null;

  return (
    <div className="space-y-1.5 rounded-md bg-base-300/40 px-2.5 py-2 text-[11px]">
      <div className="flex items-center gap-1.5 font-medium text-base-content/70">
        <Sparkles className="h-3.5 w-3.5 text-primary" />
        {t("studio.tools.border.exifAuto", {
          defaultValue: "Auto-filled from EXIF",
        })}
      </div>
      {summary.cameraLabel && (
        <div className="truncate text-base-content/60">{summary.cameraLabel}</div>
      )}
      {brandLabel && (
        <div className="flex items-center gap-1 text-base-content/50">
          <Info className="h-3 w-3 shrink-0" />
          {brandLabel}
        </div>
      )}
    </div>
  );
};

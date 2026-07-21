import React, { useCallback } from "react";
import { useI18n } from "@/lib/i18n";

type ValueSliderProps = {
  /** Already translated — this component does no key lookup. */
  label: string;
  value: number;
  defaultValue: number;
  min: number;
  max: number;
  step: number;
  unit?: string;
  /** Shows the modified dot. Callers decide what "modified" means. */
  modified?: boolean;
  disabled?: boolean;
  onChange: (value: number) => void;
};

function clamp(value: number, min: number, max: number): number {
  if (Number.isNaN(value)) return min;
  return Math.min(max, Math.max(min, value));
}

/**
 * Label, numeric input, and range track. Double-clicking the row resets it.
 *
 * Shared by the develop sliders and the frame border controls, which have
 * unrelated value models but the identical interaction — so the widget takes
 * plain numbers and a translated label rather than either feature's config
 * type.
 */
export function ValueSlider({
  label,
  value,
  defaultValue,
  min,
  max,
  step,
  unit,
  modified = false,
  disabled = false,
  onChange,
}: ValueSliderProps): React.JSX.Element {
  const { t } = useI18n();
  const format = (v: number) => (step < 1 ? Number(v.toFixed(2)) : Math.round(v));
  const reset = useCallback(() => onChange(defaultValue), [defaultValue, onChange]);

  return (
    <div
      className="py-1.5"
      onDoubleClick={reset}
      title={t("studio.develop.hint", { defaultValue: "Double-click to reset" })}
    >
      <div className="mb-1 flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <span
            className={`truncate text-[13px] ${
              modified ? "font-medium text-base-content" : "text-base-content/70"
            }`}
          >
            {label}
          </span>
          {modified && (
            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-primary" aria-label="modified" />
          )}
        </div>
        <div className="flex items-center gap-1">
          <input
            type="number"
            value={format(value)}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            aria-label={label}
            onChange={(e) => onChange(clamp(parseFloat(e.target.value), min, max))}
            className="input input-xs input-bordered h-6 w-[58px] bg-base-100 px-1.5 text-right font-mono text-[11px] tabular-nums focus:border-primary/60 focus:outline-none focus:ring-1 focus:ring-primary/60"
          />
          {unit && <span className="w-3 text-[10px] text-base-content/40">{unit}</span>}
        </div>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        aria-label={label}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        className="range range-xs range-primary"
      />
    </div>
  );
}

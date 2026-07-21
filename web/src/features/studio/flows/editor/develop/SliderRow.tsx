import React from "react";
import { useI18n } from "@/lib/i18n";
import { type AdjustmentControl, isControlModified } from "../../../model/developConfig";
import { ValueSlider } from "../../../components/ValueSlider";

type SliderRowProps = {
  control: AdjustmentControl;
  value: number;
  defaultValue: number;
  disabled?: boolean;
  onChange: (value: number) => void;
};

const SLIDER_I18N_KEYS: Record<string, string> = {
  exposure: "studio.develop.exposure",
  contrast: "studio.develop.contrast",
  highlights: "studio.develop.highlights",
  shadows: "studio.develop.shadows",
  whites: "studio.develop.whites",
  blacks: "studio.develop.blacks",
  temperature: "studio.develop.temperature",
  tint: "studio.develop.tint",
  vibrance: "studio.develop.vibrance",
  saturation: "studio.develop.saturation",
  clarity: "studio.develop.clarity",
  sharpness: "studio.develop.sharpness",
  noiseReduction: "studio.develop.noiseReduction",
};

/** Binds one develop {@link AdjustmentControl} to a {@link ValueSlider}. */
export function SliderRow({
  control,
  value,
  defaultValue,
  disabled = false,
  onChange,
}: SliderRowProps): React.JSX.Element {
  const { t } = useI18n();
  return (
    <ValueSlider
      label={t(SLIDER_I18N_KEYS[control.key] ?? `studio.develop.${control.key}`, {
        defaultValue: control.label,
      })}
      value={value}
      defaultValue={defaultValue}
      min={control.min}
      max={control.max}
      step={control.step}
      unit={control.unit}
      modified={isControlModified(control, value)}
      disabled={disabled}
      onChange={onChange}
    />
  );
}

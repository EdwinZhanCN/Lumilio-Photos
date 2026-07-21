import React from "react";
import { useI18n } from "@/lib/i18n";
import {
  DEFAULT_CANVAS,
  type CanvasBackground,
  type CanvasPad,
  type CanvasSpec,
} from "../../../model/canvasSpec";
import { ValueSlider } from "../../../components/ValueSlider";

type GradientBackground = Extract<CanvasBackground, { kind: "gradient" }>;
type FrostedBackground = Extract<CanvasBackground, { kind: "frosted" }>;

type CanvasControlsProps = {
  canvas: CanvasSpec;
  disabled?: boolean;
  onChange: (next: CanvasSpec) => void;
};

/**
 * Direct controls for the canvas treatment.
 *
 * Padding is exposed as a single "width" plus an optional extra on the bottom,
 * rather than four independent sides. Four spinners would be a worse control
 * for the thing people actually do — an even border, sometimes with a deeper
 * bottom band for a caption — and the underlying spec still supports arbitrary
 * per-side values for templates that need them.
 */
export function CanvasControls({
  canvas,
  disabled = false,
  onChange,
}: CanvasControlsProps): React.JSX.Element {
  const { t } = useI18n();

  // The even border is whatever the three non-bottom sides agree on; the bottom
  // extra is how much further the bottom band reaches.
  const evenPad = Math.min(canvas.pad.top, canvas.pad.left, canvas.pad.right);
  const bottomExtra = Math.max(0, canvas.pad.bottom - evenPad);

  const setPad = (pad: CanvasPad) => onChange({ ...canvas, pad });
  const setBackground = (background: CanvasBackground) => onChange({ ...canvas, background });

  const setEvenPad = (value: number) =>
    setPad({ top: value, left: value, right: value, bottom: value + bottomExtra });

  const setBottomExtra = (value: number) =>
    setPad({ ...canvas.pad, bottom: evenPad + value });

  return (
    <div>
      <ValueSlider
        label={t("studio.frame.padding", { defaultValue: "Border width" })}
        value={evenPad}
        defaultValue={0}
        min={0}
        max={0.3}
        step={0.005}
        modified={evenPad > 0}
        disabled={disabled}
        onChange={setEvenPad}
      />
      <ValueSlider
        label={t("studio.frame.bottomExtra", { defaultValue: "Extra at bottom" })}
        value={bottomExtra}
        defaultValue={0}
        min={0}
        max={0.4}
        step={0.005}
        modified={bottomExtra > 0}
        disabled={disabled}
        onChange={setBottomExtra}
      />

      {canvas.background.kind === "solid" && (
        <label className="flex items-center justify-between py-1.5">
          <span className="text-[13px] text-base-content/70">
            {t("studio.frame.color", { defaultValue: "Color" })}
          </span>
          <input
            type="color"
            value={canvas.background.color}
            disabled={disabled}
            onChange={(e) =>
              onChange({ ...canvas, background: { kind: "solid", color: e.target.value } })
            }
            className="h-6 w-12 cursor-pointer rounded border border-base-300 bg-base-100"
            aria-label={t("studio.frame.color", { defaultValue: "Color" })}
          />
        </label>
      )}

      {canvas.background.kind === "gradient" && (
        <GradientControls
          background={canvas.background}
          disabled={disabled}
          onChange={setBackground}
        />
      )}

      {canvas.background.kind === "frosted" && (
        <FrostedControls
          background={canvas.background}
          disabled={disabled}
          onChange={setBackground}
        />
      )}

      <ValueSlider
        label={t("studio.frame.innerRadius", { defaultValue: "Photo corners" })}
        value={canvas.innerRadius}
        defaultValue={0}
        min={0}
        max={0.2}
        step={0.005}
        modified={canvas.innerRadius > 0}
        disabled={disabled}
        onChange={(innerRadius) => onChange({ ...canvas, innerRadius })}
      />
      <ValueSlider
        label={t("studio.frame.outerRadius", { defaultValue: "Outer corners" })}
        value={canvas.outerRadius}
        defaultValue={0}
        min={0}
        max={0.2}
        step={0.005}
        modified={canvas.outerRadius > 0}
        disabled={disabled}
        onChange={(outerRadius) => onChange({ ...canvas, outerRadius })}
      />
      <ValueSlider
        label={t("studio.frame.vignette", { defaultValue: "Vignette" })}
        value={canvas.vignette}
        defaultValue={0}
        min={0}
        max={1}
        step={0.02}
        modified={canvas.vignette > 0}
        disabled={disabled}
        onChange={(vignette) => onChange({ ...canvas, vignette })}
      />

      <button
        type="button"
        className="btn btn-ghost btn-xs mt-1 w-full text-base-content/60"
        disabled={disabled}
        onClick={() => onChange({ ...DEFAULT_CANVAS, pad: { ...DEFAULT_CANVAS.pad } })}
      >
        {t("studio.frame.resetBorder", { defaultValue: "Reset border" })}
      </button>
    </div>
  );
}

function GradientControls({
  background,
  disabled,
  onChange,
}: {
  background: GradientBackground;
  disabled: boolean;
  onChange: (next: CanvasBackground) => void;
}): React.JSX.Element {
  const { t } = useI18n();
  return (
    <>
      <div className="flex items-center justify-between py-1.5">
        <span className="text-[13px] text-base-content/70">
          {t("studio.frame.gradientStops", { defaultValue: "From / to" })}
        </span>
        <div className="flex items-center gap-1">
          <input
            type="color"
            value={background.from}
            disabled={disabled}
            onChange={(e) => onChange({ ...background, from: e.target.value })}
            className="h-6 w-9 cursor-pointer rounded border border-base-300 bg-base-100"
            aria-label={t("studio.frame.gradientFrom", { defaultValue: "Gradient from" })}
          />
          <input
            type="color"
            value={background.to}
            disabled={disabled}
            onChange={(e) => onChange({ ...background, to: e.target.value })}
            className="h-6 w-9 cursor-pointer rounded border border-base-300 bg-base-100"
            aria-label={t("studio.frame.gradientTo", { defaultValue: "Gradient to" })}
          />
        </div>
      </div>
      <ValueSlider
        label={t("studio.frame.gradientAngle", { defaultValue: "Angle" })}
        value={background.angle}
        defaultValue={180}
        min={0}
        max={360}
        step={1}
        unit="°"
        modified={background.angle !== 180}
        disabled={disabled}
        onChange={(angle) => onChange({ ...background, angle })}
      />
    </>
  );
}

function FrostedControls({
  background,
  disabled,
  onChange,
}: {
  background: FrostedBackground;
  disabled: boolean;
  onChange: (next: CanvasBackground) => void;
}): React.JSX.Element {
  const { t } = useI18n();
  return (
    <>
      <ValueSlider
        label={t("studio.frame.blur", { defaultValue: "Blur" })}
        value={background.blur}
        defaultValue={0.06}
        min={0}
        max={0.2}
        step={0.005}
        modified={background.blur !== 0.06}
        disabled={disabled}
        onChange={(blur) => onChange({ ...background, blur })}
      />
      <ValueSlider
        label={t("studio.frame.brightness", { defaultValue: "Brightness" })}
        value={background.brightness}
        defaultValue={-0.16}
        min={-1}
        max={1}
        step={0.02}
        modified={background.brightness !== -0.16}
        disabled={disabled}
        onChange={(brightness) => onChange({ ...background, brightness })}
      />
    </>
  );
}

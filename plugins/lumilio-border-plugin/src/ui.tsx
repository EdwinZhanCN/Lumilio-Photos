import React from "react";
import { DEFAULT_PARAMS, type BorderParams, normalizeParams } from "./types";

export const meta = {
  id: "com.lumilio.border",
  version: "0.1.0",
  displayName: "Lumilio Border",
  mount: {
    panel: "frames" as const,
    order: 10,
  },
};

export const defaultParams: Record<string, unknown> = DEFAULT_PARAMS;

const BorderPanel: React.FC<{
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}> = ({ value, onChange, disabled }) => {
  const p = normalizeParams(value);

  const update = (next: Partial<BorderParams>) => {
    onChange({ ...p, ...next });
  };

  return (
    <div className="space-y-4">
      <div className="tabs tabs-boxed">
        <button
          type="button"
          className={`tab ${p.mode === "COLORED" ? "tab-active" : ""}`}
          disabled={disabled}
          onClick={() => update({ mode: "COLORED" })}
        >
          Colored
        </button>
        <button
          type="button"
          className={`tab ${p.mode === "FROSTED" ? "tab-active" : ""}`}
          disabled={disabled}
          onClick={() => update({ mode: "FROSTED" })}
        >
          Frosted
        </button>
        <button
          type="button"
          className={`tab ${p.mode === "VIGNETTE" ? "tab-active" : ""}`}
          disabled={disabled}
          onClick={() => update({ mode: "VIGNETTE" })}
        >
          Vignette
        </button>
      </div>

      {p.mode === "COLORED" && (
        <div className="space-y-2">
          <label className="label">Border Width: {p.border_width}</label>
          <input
            className="range range-primary"
            type="range"
            min={1}
            max={100}
            value={p.border_width}
            disabled={disabled}
            onChange={(e) => update({ border_width: Number(e.target.value) })}
          />

          <label className="label">Color</label>
          <input
            className="w-full h-10 p-1"
            type="color"
            value={p.color_hex}
            disabled={disabled}
            onChange={(e) => update({ color_hex: e.target.value })}
          />
        </div>
      )}

      {p.mode === "FROSTED" && (
        <div className="space-y-2">
          <label className="label">Blur: {p.blur_sigma.toFixed(1)}</label>
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

          <label className="label">Brightness: {p.brightness_adjustment}</label>
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

          <label className="label">Corner Radius: {p.corner_radius}</label>
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
          <label className="label">Strength: {p.strength.toFixed(2)}</label>
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

      <div className="space-y-2">
        <label className="label">JPEG Quality: {p.jpeg_quality}</label>
        <input
          className="range range-primary"
          type="range"
          min={1}
          max={100}
          value={p.jpeg_quality}
          disabled={disabled}
          onChange={(e) => update({ jpeg_quality: Number(e.target.value) })}
        />
      </div>
    </div>
  );
};

export const Panel = BorderPanel;
export { normalizeParams };

export default {
  meta,
  defaultParams,
  Panel,
  normalizeParams,
};

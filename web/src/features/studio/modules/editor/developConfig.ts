import { Focus, Palette, Sun, type LucideIcon } from "lucide-react";
import { DEFAULT_STUDIO_ADJUSTMENTS, type StudioEditAdjustments } from "./runtime/types";

/**
 * Photometric adjustment keys — the numeric subset of {@link StudioEditAdjustments}
 * that the WebGPU/WebGL2 develop shader consumes. Geometry (rotation/flip) and
 * `crop` are handled separately.
 *
 * NOTE: ranges and defaults intentionally mirror the shader contract
 * (`adjustments.<x> / 100.0`, exposure in stops, `0` == no-op). The design
 * handoff's Kelvin/temperature and non-zero defaults are NOT used because they
 * would diverge from the real pipeline and break the "defaults == original"
 * fast path in the worker.
 */
export type AdjustmentKey =
  | "exposure"
  | "contrast"
  | "highlights"
  | "shadows"
  | "whites"
  | "blacks"
  | "temperature"
  | "tint"
  | "vibrance"
  | "saturation"
  | "clarity"
  | "sharpness"
  | "noiseReduction";

export interface AdjustmentControl {
  key: AdjustmentKey;
  label: string;
  min: number;
  max: number;
  step: number;
  unit?: string;
}

export interface DevelopGroup {
  id: "light" | "color" | "detail";
  title: string;
  icon: LucideIcon;
  controls: AdjustmentControl[];
}

export const DEVELOP_GROUPS: DevelopGroup[] = [
  {
    id: "light",
    title: "Light",
    icon: Sun,
    controls: [
      { key: "exposure", label: "Exposure", min: -3, max: 3, step: 0.05 },
      { key: "contrast", label: "Contrast", min: -100, max: 100, step: 1 },
      { key: "highlights", label: "Highlights", min: -100, max: 100, step: 1 },
      { key: "shadows", label: "Shadows", min: -100, max: 100, step: 1 },
      { key: "whites", label: "Whites", min: -100, max: 100, step: 1 },
      { key: "blacks", label: "Blacks", min: -100, max: 100, step: 1 },
    ],
  },
  {
    id: "color",
    title: "Color",
    icon: Palette,
    controls: [
      { key: "temperature", label: "Temperature", min: -100, max: 100, step: 1 },
      { key: "tint", label: "Tint", min: -100, max: 100, step: 1 },
      { key: "vibrance", label: "Vibrance", min: -100, max: 100, step: 1 },
      { key: "saturation", label: "Saturation", min: -100, max: 100, step: 1 },
    ],
  },
  {
    id: "detail",
    title: "Detail",
    icon: Focus,
    controls: [
      { key: "clarity", label: "Clarity", min: -100, max: 100, step: 1 },
      { key: "sharpness", label: "Sharpness", min: 0, max: 100, step: 1 },
      {
        key: "noiseReduction",
        label: "Noise Reduction",
        min: 0,
        max: 100,
        step: 1,
      },
    ],
  },
];

/** True when the control differs from its (zero) default within step tolerance. */
export function isControlModified(control: AdjustmentControl, value: number): boolean {
  const def = DEFAULT_STUDIO_ADJUSTMENTS[control.key];
  const tolerance = control.step < 1 ? 0.0001 : 0.5;
  return Math.abs(value - def) > tolerance;
}

/** True when any control in the group is modified. */
export function isGroupModified(group: DevelopGroup, adjustments: StudioEditAdjustments): boolean {
  return group.controls.some((control) => isControlModified(control, adjustments[control.key]));
}

export function isGeometryModified(adjustments: StudioEditAdjustments): boolean {
  return adjustments.rotation !== 0 || adjustments.flipHorizontal || adjustments.flipVertical;
}

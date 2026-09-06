import { exportLongEdge, type PhotoExportSize } from "@/lib/photo-export/model";
import { resolveCanvasGeometry, type CanvasSpec } from "../../model/canvasSpec";
import type { StudioEditAdjustments } from "../../model/editTypes";
import { deriveRenderSize } from "./coordinateSystem";

export function composedExportDimensions(
  width: number,
  height: number,
  adjustments: StudioEditAdjustments,
  canvas: CanvasSpec | null,
) {
  const size = deriveRenderSize(
    width,
    height,
    adjustments.rotation,
    adjustments.crop,
    Math.max(width, height),
  );
  if (!canvas) return { width: size.outWidth, height: size.outHeight };
  const frame = resolveCanvasGeometry(size.outWidth, size.outHeight, canvas);
  return { width: frame.outWidth, height: frame.outHeight };
}

/** Size choices describe the final framed photo, not the inner image. */
export function studioExportSize(
  width: number,
  height: number,
  adjustments: StudioEditAdjustments,
  canvas: CanvasSpec | null,
  mode: PhotoExportSize,
  maxDimension: number,
) {
  const native = composedExportDimensions(width, height, adjustments, canvas);
  const nativeLongEdge = Math.max(native.width, native.height);
  const requested = exportLongEdge(mode, nativeLongEdge);
  const outputLongEdge = Math.min(requested, maxDimension);
  const photoLongEdge = Math.max(
    adjustments.crop?.width ?? width,
    adjustments.crop?.height ?? height,
  );
  return {
    nativeLongEdge,
    outputLongEdge,
    renderLongEdge: Math.max(1, Math.floor((photoLongEdge * outputLongEdge) / nativeLongEdge)),
    downscaled: outputLongEdge < requested,
  };
}

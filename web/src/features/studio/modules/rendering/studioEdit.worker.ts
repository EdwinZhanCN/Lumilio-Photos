/**
 * The Studio render worker.
 *
 * It owns the on-screen preview canvas (handed over once via
 * `transferControlToOffscreen`) and the persistent {@link DevelopEngine}. A
 * preview render develops the photo on the GPU, applies geometry and
 * composition on a 2D canvas, and draws the result straight onto the visible
 * canvas — no blob, no objectURL, no GPU→CPU readback. Only export produces a
 * blob, at the chosen output resolution.
 *
 * Pipeline: source texture (once) → develop (uniforms only) → crop/rotate/flip
 * → border + layers → blit to the visible canvas.
 */

import {
  DEFAULT_STUDIO_ADJUSTMENTS,
  normalizeStudioAdjustments,
  type StudioEditAdjustments,
} from "../../model/editTypes";
import { composeStudioImage, type Composition } from "./composeStudioImage";
import { deriveRenderSize, resolveExportSize, type ExportSizeMode } from "./coordinateSystem";
import { DevelopEngine } from "./developEngine";
import { ensureStudioFontsLoaded } from "./fonts/loadStudioFonts";
import { applyGeometry } from "./geometry";
import { context2d } from "./canvasUtils";

type RenderEngineName = "webgl2" | "webgpu";

/**
 * Rasterized brand marks, keyed as `renderLayers` looks them up. The worker
 * cannot decode SVG, so the main thread rasterizes and transfers bitmaps here.
 */
let logoImages = new Map<string, ImageBitmap>();

let engine: DevelopEngine | null = null;
let sourceBitmap: ImageBitmap | null = null;
let visibleCanvas: OffscreenCanvas | null = null;
/** Reused geometry target so a dragging slider does not allocate every frame. */
let geometryCanvas: OffscreenCanvas | null = null;
/** Scene depth field for layer occlusion, and its feather width. */
let depthField: { data: Uint8ClampedArray; width: number; height: number } | null = null;
let depthFeather = 0.08;

type InitCanvasMessage = {
  type: "INIT_CANVAS";
  payload: { requestId: number; canvas: OffscreenCanvas };
};

type LoadImageMessage = {
  type: "LOAD_IMAGE";
  payload: {
    requestId: number;
    blob: Blob;
    adjustments?: Partial<StudioEditAdjustments>;
    composition?: Composition;
    previewMaxSize?: number;
    snapshotMaxSize?: number;
  };
};

type RenderMessage = {
  type: "RENDER";
  payload: {
    requestId: number;
    adjustments: Partial<StudioEditAdjustments>;
    composition?: Composition;
    previewMaxSize?: number;
    depthFeather?: number;
  };
};

type ExportImageMessage = {
  type: "EXPORT_IMAGE";
  payload: {
    requestId: number;
    adjustments: Partial<StudioEditAdjustments>;
    composition?: Composition;
    format: "image/jpeg" | "image/png" | "image/webp";
    quality: number;
    sizeMode: ExportSizeMode;
  };
};

type SetLogosMessage = {
  type: "SET_LOGOS";
  payload: { requestId: number; logos: Array<[string, ImageBitmap]> };
};

/** Hands over the scene depth field (grayscale), or null to clear it. */
type SetDepthMessage = {
  type: "SET_DEPTH";
  payload: { requestId: number; depth: ImageBitmap | null; feather?: number };
};

/**
 * Returns the developed + geometry photo (crop/rotate/flip applied, no border or
 * layers) as a blob — the input depth estimation must run on so the field aligns
 * with the edited image rather than the raw source.
 */
type SnapshotMessage = {
  type: "SNAPSHOT";
  payload: { requestId: number; adjustments: Partial<StudioEditAdjustments>; maxSize?: number };
};

type WorkerMessage =
  | InitCanvasMessage
  | LoadImageMessage
  | RenderMessage
  | ExportImageMessage
  | SetLogosMessage
  | SetDepthMessage
  | SnapshotMessage;

function requireEngine(): DevelopEngine {
  if (!engine) throw new Error("No source image loaded");
  return engine;
}

async function ensureFontsForComposition(composition: Composition | undefined): Promise<void> {
  const hasText = composition?.layers.some((layer) => layer.type === "text") ?? false;
  if (hasText) await ensureStudioFontsLoaded();
}

/**
 * Develop + geometry + composition into an OffscreenCanvas at the given budget.
 * `reuseGeometry` lets the preview path recycle its intermediate canvas.
 */
function renderComposed(
  adjustments: StudioEditAdjustments,
  composition: Composition | undefined,
  maxSize: number,
  reuseGeometry: boolean,
): OffscreenCanvas {
  const active = requireEngine();
  const size = deriveRenderSize(
    active.sourceWidth,
    active.sourceHeight,
    adjustments.rotation,
    adjustments.crop,
    maxSize,
  );

  const developed = active.render(adjustments, size.developWidth, size.developHeight);
  const geometry = applyGeometry(
    developed,
    {
      crop: adjustments.crop,
      rotation: adjustments.rotation,
      flipHorizontal: adjustments.flipHorizontal,
      flipVertical: adjustments.flipVertical,
      scale: size.scale,
    },
    reuseGeometry ? geometryCanvas : null,
  );
  if (reuseGeometry) geometryCanvas = geometry;

  if (!composition) return geometry;
  const occlusion = depthField ? { field: depthField, feather: depthFeather } : undefined;
  return composeStudioImage(geometry, composition, logoImages, occlusion);
}

/** Draw a composed canvas onto the visible (transferred) preview canvas. */
function blitToVisible(composed: OffscreenCanvas): { outWidth: number; outHeight: number } {
  if (!visibleCanvas) throw new Error("Preview canvas is not connected");
  if (visibleCanvas.width !== composed.width) visibleCanvas.width = composed.width;
  if (visibleCanvas.height !== composed.height) visibleCanvas.height = composed.height;
  const ctx = context2d(visibleCanvas);
  ctx.clearRect(0, 0, composed.width, composed.height);
  ctx.drawImage(composed, 0, 0);
  return { outWidth: composed.width, outHeight: composed.height };
}

async function renderPreview(
  adjustmentsInput: Partial<StudioEditAdjustments> | undefined,
  composition: Composition | undefined,
  previewMaxSize: number,
): Promise<{ outWidth: number; outHeight: number; engine: RenderEngineName }> {
  const adjustments = normalizeStudioAdjustments(adjustmentsInput ?? DEFAULT_STUDIO_ADJUSTMENTS);
  await ensureFontsForComposition(composition);
  const composed = renderComposed(adjustments, composition, previewMaxSize, true);
  const size = blitToVisible(composed);
  return { ...size, engine: "webgl2" };
}

self.onmessage = async (event: MessageEvent<WorkerMessage>) => {
  try {
    const message = event.data;
    const requestId = message.payload.requestId;

    if (message.type === "INIT_CANVAS") {
      visibleCanvas = message.payload.canvas;
      self.postMessage({ type: "CANVAS_READY", payload: { requestId } });
      return;
    }

    if (message.type === "SET_LOGOS") {
      for (const bitmap of logoImages.values()) bitmap.close();
      logoImages = new Map(message.payload.logos);
      self.postMessage({ type: "LOGOS_SET", payload: { requestId } });
      return;
    }

    if (message.type === "SET_DEPTH") {
      if (typeof message.payload.feather === "number") depthFeather = message.payload.feather;
      const bitmap = message.payload.depth;
      if (!bitmap) {
        depthField = null;
      } else {
        const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
        const ctx = context2d(canvas);
        ctx.drawImage(bitmap, 0, 0);
        depthField = {
          data: ctx.getImageData(0, 0, bitmap.width, bitmap.height).data,
          width: bitmap.width,
          height: bitmap.height,
        };
        bitmap.close();
      }
      self.postMessage({ type: "DEPTH_SET", payload: { requestId } });
      return;
    }

    if (message.type === "SNAPSHOT") {
      const adjustments = normalizeStudioAdjustments(message.payload.adjustments);
      const photo = renderComposed(adjustments, undefined, message.payload.maxSize ?? 1024, false);
      const blob = await photo.convertToBlob({ type: "image/png" });
      self.postMessage({ type: "SNAPSHOT_COMPLETE", payload: { requestId, blob } });
      return;
    }

    if (message.type === "LOAD_IMAGE") {
      engine?.dispose();
      sourceBitmap?.close();
      geometryCanvas = null;
      sourceBitmap = await createImageBitmap(message.payload.blob);
      engine = DevelopEngine.create(sourceBitmap);

      const adjustments = normalizeStudioAdjustments(message.payload.adjustments);
      await ensureFontsForComposition(message.payload.composition);
      const composed = renderComposed(
        adjustments,
        message.payload.composition,
        message.payload.previewMaxSize ?? 1800,
        true,
      );
      const size = blitToVisible(composed);

      // A developed-only snapshot feeds the frame template previews and adaptive
      // overlay ink on the main thread, which needs real pixels but no DOM.
      const snapshotSource = renderComposed(
        adjustments,
        undefined,
        message.payload.snapshotMaxSize ?? 1024,
        false,
      );
      const snapshot = await createImageBitmap(snapshotSource);

      self.postMessage(
        {
          type: "IMAGE_LOADED",
          payload: {
            requestId,
            originalWidth: engine.originalSourceWidth,
            originalHeight: engine.originalSourceHeight,
            // Effective (post-guardrail) source — the space crop rectangles and
            // the render pipeline share.
            sourceWidth: engine.sourceWidth,
            sourceHeight: engine.sourceHeight,
            outWidth: size.outWidth,
            outHeight: size.outHeight,
            engine: "webgl2",
            snapshot,
          },
        },
        [snapshot],
      );
      return;
    }

    if (message.type === "RENDER") {
      if (typeof message.payload.depthFeather === "number") {
        depthFeather = message.payload.depthFeather;
      }
      const result = await renderPreview(
        message.payload.adjustments,
        message.payload.composition,
        message.payload.previewMaxSize ?? 1800,
      );
      self.postMessage({ type: "RENDER_COMPLETE", payload: { requestId, ...result } });
      return;
    }

    if (message.type === "EXPORT_IMAGE") {
      const active = requireEngine();
      const adjustments = normalizeStudioAdjustments(message.payload.adjustments);
      await ensureFontsForComposition(message.payload.composition);

      const plan = resolveExportSize(
        active.sourceWidth,
        active.sourceHeight,
        adjustments.crop,
        message.payload.sizeMode,
        active.maxTextureSize,
      );

      // Render at the planned size; if a browser rejects the resulting canvas as
      // too large, back off by halves and report the export as downscaled.
      let maxSize = plan.maxSize;
      let downscaled = plan.downscaled;
      let composed: OffscreenCanvas | null = null;
      let blob: Blob | null = null;
      for (;;) {
        try {
          composed = renderComposed(adjustments, message.payload.composition, maxSize, false);
          blob = await composed.convertToBlob({
            type: message.payload.format,
            quality: message.payload.quality,
          });
          break;
        } catch (renderError) {
          if (maxSize <= 512) throw renderError;
          maxSize = Math.floor(maxSize / 2);
          downscaled = true;
        }
      }

      self.postMessage({
        type: "EXPORT_COMPLETE",
        payload: {
          requestId,
          blob,
          width: composed.width,
          height: composed.height,
          downscaled,
          nativeLongEdge: plan.nativeLongEdge,
        },
      });
      return;
    }
  } catch (error) {
    self.postMessage({
      type: "ERROR",
      payload: {
        requestId: event.data.payload.requestId,
        error: error instanceof Error ? error.message : "Studio edit worker failed",
      },
    });
  }
};

export {};

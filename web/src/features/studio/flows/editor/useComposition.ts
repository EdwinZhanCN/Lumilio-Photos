import { useCallback, useEffect, useRef, useState } from "react";
import type { CanvasSpec } from "../../model/canvasSpec";
import type { Layer } from "../../model/layers";
import type { StudioComposition } from "../../model/editTypes";
import { applyTemplate } from "../../modules/frame/applyTemplate";
import { findTemplate, FRAME_TEMPLATES } from "../../modules/frame/frameTemplates";
import { collectTemplateLogos, renderTemplatePreviews } from "../../modules/frame/templatePreviews";
import { rasterizeLogos } from "../../modules/frame/logoRaster";
import { hasSufficientExif, type FrameExif } from "../../modules/frame/frameExif";
import { context2d, createCanvas } from "../../modules/rendering/canvasUtils";
import { ensureStudioFontsLoaded } from "../../modules/rendering/fonts/loadStudioFonts";

type UseCompositionInput = {
  /** The developed photo, used for template previews and adaptive ink. */
  photo: ImageBitmap | null;
  exif: FrameExif;
  /** Receives freshly rasterized marks to hand to the render worker. */
  onLogosReady: (logos: Map<string, ImageBitmap>) => void;
};

export type CompositionController = {
  canvas: CanvasSpec | null;
  layers: Layer[];
  activeTemplateId: string | null;
  selectedLayerId: string | null;
  templatePreviews: ReadonlyMap<string, string>;
  exifAvailable: boolean;

  setCanvas: (next: CanvasSpec) => void;
  setLayers: (next: Layer[]) => void;
  setSelectedLayerId: (id: string | null) => void;
  applyTemplateById: (templateId: string) => Promise<void>;
  clearTemplate: () => void;
  /** Replaces the whole composition, e.g. when a sidecar loads. */
  reset: (composition: StudioComposition) => void;
};

/**
 * Owns the composition half of a Studio edit: the canvas treatment, the layer
 * stack, and the template previews that let someone choose between presets.
 *
 * Logo rasterization lives here rather than in the worker because decoding SVG
 * needs the DOM. The bitmaps are handed upward so the editor can transfer them
 * to the render worker once, instead of per frame.
 */
export function useComposition({
  photo,
  exif,
  onLogosReady,
}: UseCompositionInput): CompositionController {
  const [canvas, setCanvasState] = useState<CanvasSpec | null>(null);
  const [layers, setLayersState] = useState<Layer[]>([]);
  const [activeTemplateId, setActiveTemplateId] = useState<string | null>(null);
  const [selectedLayerId, setSelectedLayerId] = useState<string | null>(null);
  const [templatePreviews, setTemplatePreviews] = useState<ReadonlyMap<string, string>>(new Map());

  const previewUrlsRef = useRef<ReadonlyMap<string, string>>(new Map());
  const logosRef = useRef<Map<string, ImageBitmap>>(new Map());

  const revokePreviews = useCallback(() => {
    for (const url of previewUrlsRef.current.values()) URL.revokeObjectURL(url);
    previewUrlsRef.current = new Map();
  }, []);

  useEffect(() => revokePreviews, [revokePreviews]);

  // Rasterize every mark the catalog needs for this photo, then render one
  // preview per template. Both depend only on the photo and its EXIF, so this
  // does not re-run while the user edits.
  useEffect(() => {
    if (!photo) return;
    let alive = true;

    void (async () => {
      try {
        await ensureStudioFontsLoaded();
        if (!alive) return;

        const requests = collectTemplateLogos(FRAME_TEMPLATES, exif, photo.width, photo.height);
        const logos = await rasterizeLogos(requests);
        if (!alive) return;

        logosRef.current = logos;
        // The worker needs its own copies: transferring detaches ours.
        onLogosReady(await rasterizeLogos(requests));

        const urls = await renderTemplatePreviews({
          photo,
          exif,
          templates: FRAME_TEMPLATES,
          logos,
        });
        if (!alive) {
          for (const url of urls.values()) URL.revokeObjectURL(url);
          return;
        }
        revokePreviews();
        previewUrlsRef.current = urls;
        setTemplatePreviews(urls);
      } catch (error) {
        // Previews are an affordance, not the feature: a failure here leaves
        // placeholder tiles and template application still works.
        console.warn("[studio] template previews unavailable", error);
      }
    })();

    return () => {
      alive = false;
    };
  }, [photo, exif, onLogosReady, revokePreviews]);

  const applyTemplateById = useCallback(
    async (templateId: string) => {
      const template = findTemplate(templateId);
      if (!template || !photo) return;

      await ensureStudioFontsLoaded();
      const measureCtx = context2d(createCanvas(1, 1));
      const expanded = applyTemplate(template, { photo, exif, measureCtx });

      // Marks this template needs may not be in the worker's set yet.
      if (expanded.logoRequests.length > 0) {
        onLogosReady(await rasterizeLogos(expanded.logoRequests));
      }

      // Replace template-produced layers, keep hand-authored ones on top.
      setLayersState((current) => [
        ...expanded.layers,
        ...current.filter((layer) => !layer.fromTemplate),
      ]);
      setCanvasState(expanded.canvas);
      setActiveTemplateId(templateId);
    },
    [photo, exif, onLogosReady],
  );

  const clearTemplate = useCallback(() => {
    setLayersState((current) => current.filter((layer) => !layer.fromTemplate));
    setCanvasState(null);
    setActiveTemplateId(null);
  }, []);

  const setCanvas = useCallback((next: CanvasSpec) => {
    setCanvasState(next);
    // Hand-editing the border detaches it from the preset it came from.
    setActiveTemplateId(null);
  }, []);

  const setLayers = useCallback((next: Layer[]) => setLayersState(next), []);

  const reset = useCallback((composition: StudioComposition) => {
    setCanvasState(composition.canvas);
    setLayersState(composition.layers);
    setActiveTemplateId(null);
    setSelectedLayerId(null);
  }, []);

  return {
    canvas,
    layers,
    activeTemplateId,
    selectedLayerId,
    templatePreviews,
    exifAvailable: hasSufficientExif(exif),
    setCanvas,
    setLayers,
    setSelectedLayerId,
    applyTemplateById,
    clearTemplate,
    reset,
  };
}

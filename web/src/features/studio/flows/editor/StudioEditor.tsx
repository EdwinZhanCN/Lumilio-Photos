import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import client from "@/lib/http-commons/client";
import { useI18n } from "@/lib/i18n";
import { useMessage } from "@/features/notifications";
import { extractFrameExif, type FrameExif } from "../../modules/frame/frameExif";
import {
  DEFAULT_STUDIO_ADJUSTMENTS,
  EMPTY_COMPOSITION,
  normalizeStudioComposition,
  normalizeStudioAdjustments,
  type AssetSidecarResponse,
  type LumilioSidecarV1,
  type StudioComposition,
  type StudioEditAdjustments,
} from "../../model/editTypes";
import type { AdjustmentKey } from "../../model/developConfig";
import { TopBar } from "./TopBar";
import { AssetPanel, type AssetExifRow } from "./AssetPanel";
import { Viewport } from "./Viewport";
import { EditorPanel, type EditorTab } from "./EditorPanel";
import { useComposition } from "./useComposition";

type WorkerResult = {
  blob: Blob;
  width: number;
  height: number;
  engine?: "webgpu" | "webgl2" | "wasm-cpu" | "canvas-2d";
  originalWidth?: number;
  originalHeight?: number;
};

type WorkerSuccessType = "IMAGE_LOADED" | "PREVIEW_COMPLETE" | "EXPORT_COMPLETE";

export type StudioEditorActivity = {
  assetId: string;
  name: string;
  width: number | null;
  height: number | null;
};

type StudioEditorProps = {
  assetId: string;
  onBack: () => void;
  onActivity?: (activity: StudioEditorActivity) => void;
  /** Open the Frame tab on entry, e.g. arriving from a 'add a frame' action. */
  focusFrame?: boolean;
};

const apiClient = client as typeof client & {
  GET: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
  PUT: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
};

function getStudioSourceUrl(asset: Asset, assetId: string): string {
  return assetUrls.getExportUrl(assetId, {
    format: "jpeg",
    quality: 95,
    maxWidth: 4096,
    maxHeight: 4096,
    filename: (asset.original_filename ?? "studio-source").replace(/\.[^.]+$/, ""),
  });
}

function getAssetDimension(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;
}

// ---------------------------------------------------------------------------
// Pure helpers (ported from the original Edit MVP route)
// ---------------------------------------------------------------------------
function isAsset(value: unknown): value is Asset {
  return Boolean(
    value &&
    typeof value === "object" &&
    typeof (value as Record<string, unknown>).asset_id === "string",
  );
}

function unwrapData<T>(response: unknown, guard: (value: unknown) => value is T): T | undefined {
  if (guard(response)) return response;
  return undefined;
}

function isSidecarResponse(value: unknown): value is AssetSidecarResponse {
  return Boolean(
    value &&
    typeof value === "object" &&
    typeof (value as Record<string, unknown>).asset_id === "string" &&
    typeof (value as Record<string, unknown>).sidecar === "object",
  );
}

function isExifResponse(value: unknown): value is { exif_raw: Record<string, unknown> } {
  return Boolean(
    value &&
    typeof value === "object" &&
    typeof (value as Record<string, unknown>).exif_raw === "object",
  );
}

function getExifValue(exif: Record<string, unknown>, keys: string[]): string | undefined {
  for (const key of keys) {
    const value = exif[key];
    if (value === null || value === undefined || value === "") continue;
    if (Array.isArray(value)) return value.join(", ");
    if (typeof value === "string") return value;
    if (typeof value === "number" || typeof value === "boolean") return String(value);
    return JSON.stringify(value) ?? "";
  }
  return undefined;
}

function filterExifRows(exif: Record<string, unknown> | null): AssetExifRow[] {
  if (!exif) return [];
  const fields: Array<[string, string[]]> = [
    ["Camera", ["Model", "CameraModelName", "Make", "EXIF:Model", "Exif.Image.Model"]],
    ["Lens", ["LensModel", "Lens", "EXIF:LensModel", "Exif.Photo.LensModel"]],
    ["ISO", ["ISO", "Exif.Photo.ISOSpeedRatings", "EXIF:ISO"]],
    ["Aperture", ["Aperture", "FNumber", "EXIF:FNumber", "Exif.Photo.FNumber"]],
    ["Shutter", ["ShutterSpeed", "ExposureTime", "EXIF:ExposureTime", "Exif.Photo.ExposureTime"]],
    ["Focal Length", ["FocalLength", "EXIF:FocalLength", "Exif.Photo.FocalLength"]],
    [
      "Captured",
      ["DateTimeOriginal", "CreateDate", "EXIF:DateTimeOriginal", "Exif.Photo.DateTimeOriginal"],
    ],
  ];
  return fields.flatMap(([label, keys]) => {
    const value = getExifValue(exif, keys);
    return value ? [{ label, value }] : [];
  });
}

function createSidecar(
  asset: Asset,
  adjustments: StudioEditAdjustments,
  composition: StudioComposition,
): LumilioSidecarV1 {
  return {
    version: 1,
    asset_id: asset.asset_id ?? "",
    source: {
      original_filename: asset.original_filename ?? "",
      storage_path: asset.storage_path ?? "",
      mime_type: asset.mime_type ?? "",
      file_size: asset.file_size ?? 0,
      hash: asset.hash ?? null,
      width: asset.width ?? null,
      height: asset.height ?? null,
    },
    adjustments,
    canvas: composition.canvas,
    layers: composition.layers,
    updated_at: new Date().toISOString(),
  };
}

function formatBytes(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return "-";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value < 10 && unit > 0 ? 1 : 0)} ${units[unit]}`;
}

function triggerDownload(url: string, fileName: string): void {
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
}

async function decodeBlobToImageData(
  blob: Blob,
  maxSize: number,
): Promise<{ imageData: ImageData; originalWidth: number; originalHeight: number }> {
  const url = URL.createObjectURL(blob);
  try {
    const image = new Image();
    image.decoding = "async";
    image.src = url;
    await image.decode();

    const originalWidth = image.naturalWidth;
    const originalHeight = image.naturalHeight;
    const scale = Math.min(1, maxSize / Math.max(originalWidth, originalHeight));
    const width = Math.max(1, Math.round(originalWidth * scale));
    const height = Math.max(1, Math.round(originalHeight * scale));
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) throw new Error("Canvas decode context is not available");
    ctx.drawImage(image, 0, 0, width, height);
    return {
      imageData: ctx.getImageData(0, 0, width, height),
      originalWidth,
      originalHeight,
    };
  } finally {
    URL.revokeObjectURL(url);
  }
}

function cloneImageData(imageData: ImageData): ImageData {
  return new ImageData(new Uint8ClampedArray(imageData.data), imageData.width, imageData.height);
}

// ===========================================================================
// StudioEditor
// ===========================================================================
export function StudioEditor({
  assetId,
  onBack,
  onActivity,
  focusFrame = false,
}: StudioEditorProps): React.JSX.Element {
  const { t } = useI18n();
  const showMessage = useMessage();

  // Worker + source refs
  const workerRef = useRef<Worker | null>(null);
  const workerRequestIdRef = useRef(0);
  const workerHasSourceRef = useRef(false);
  const sourceImageDataRef = useRef<ImageData | null>(null);
  const sourceOriginalSizeRef = useRef<{ width: number; height: number } | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const originalPreviewUrlRef = useRef<string | null>(null);
  const lastSavedSignatureRef = useRef(
    JSON.stringify({ adjustments: DEFAULT_STUDIO_ADJUSTMENTS, composition: EMPTY_COMPOSITION }),
  );
  const skipNextRenderRef = useRef(false);
  const renderGenerationRef = useRef(0);
  const savedTimerRef = useRef<number | null>(null);

  // State
  const [asset, setAsset] = useState<Asset | null>(null);
  const [adjustments, setAdjustments] = useState<StudioEditAdjustments>(DEFAULT_STUDIO_ADJUSTMENTS);
  const [history, setHistory] = useState<StudioEditAdjustments[]>([]);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [originalPreviewUrl, setOriginalPreviewUrl] = useState<string | null>(null);
  const [showOriginal, setShowOriginal] = useState(false);
  const [mobileDevelopOpen, setMobileDevelopOpen] = useState(false);
  const [imageSize, setImageSize] = useState<{ width: number; height: number } | null>(null);
  const [exifRows, setExifRows] = useState<AssetExifRow[]>([]);
  const [isLoadingAsset, setIsLoadingAsset] = useState(false);
  const [isRendering, setIsRendering] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [renderEngine, setRenderEngine] = useState<WorkerResult["engine"]>(undefined);
  const [error, setError] = useState<string | null>(null);

  // Composition
  const [tab, setTab] = useState<EditorTab>(focusFrame ? "frame" : "develop");
  const [frameExif, setFrameExif] = useState<FrameExif>({});
  const [photoBitmap, setPhotoBitmap] = useState<ImageBitmap | null>(null);

  const sendLogosToWorker = useCallback((logos: Map<string, ImageBitmap>) => {
    if (logos.size === 0) return;
    const worker = ensureWorker();
    const entries = Array.from(logos.entries());
    worker.postMessage(
      { type: "SET_LOGOS", payload: { requestId: ++workerRequestIdRef.current, logos: entries } },
      entries.map(([, bitmap]) => bitmap),
    );
  }, []);

  const composition = useComposition({
    photo: photoBitmap,
    exif: frameExif,
    onLogosReady: sendLogosToWorker,
  });

  // The asset-load effect restores a saved composition, but must not re-run
  // when the composition changes — that would reload the photo on every edit.
  const compositionRef = useRef(composition);
  compositionRef.current = composition;

  // Dirty tracking spans both halves of the edit: a moved caption is as much
  // an unsaved change as a moved slider.
  const currentComposition = useMemo<StudioComposition>(
    () => ({ canvas: composition.canvas, layers: composition.layers }),
    [composition.canvas, composition.layers],
  );
  const currentSignature = useMemo(
    () => JSON.stringify({ adjustments, composition: currentComposition }),
    [adjustments, currentComposition],
  );
  const isDirty = currentSignature !== lastSavedSignatureRef.current;

  const baseAspect = useMemo(() => {
    if (imageSize && imageSize.width > 0 && imageSize.height > 0) {
      return imageSize.width / imageSize.height;
    }
    return 3 / 2;
  }, [imageSize]);

  // ----- Worker plumbing -----
  const ensureWorker = useCallback(() => {
    if (!workerRef.current) {
      workerRef.current = new Worker(
        new URL("../../modules/rendering/studioEdit.worker.ts", import.meta.url),
        {
        type: "module",
        },
      );
      workerHasSourceRef.current = false;
    }
    return workerRef.current;
  }, []);

  const callWorker = useCallback(
    <T extends WorkerResult>(
      type: "LOAD_IMAGE_DATA" | "RENDER_PREVIEW" | "EXPORT_IMAGE",
      payload: Record<string, unknown>,
      successType: WorkerSuccessType,
    ): Promise<T> => {
      const activeWorker = ensureWorker();
      const requestId = ++workerRequestIdRef.current;

      return new Promise<T>((resolve, reject) => {
        const timeoutId = window.setTimeout(() => {
          activeWorker.removeEventListener("message", onMessage);
          activeWorker.terminate();
          if (workerRef.current === activeWorker) {
            workerRef.current = null;
            workerHasSourceRef.current = false;
          }
          reject(new Error("Studio worker timed out"));
        }, 20000);

        const onMessage = (event: MessageEvent) => {
          const { type: eventType, payload: eventPayload } = event.data || {};
          if (!eventPayload || eventPayload.requestId !== requestId) return;
          if (eventType === successType) {
            window.clearTimeout(timeoutId);
            activeWorker.removeEventListener("message", onMessage);
            resolve(eventPayload as T);
            return;
          }
          if (eventType === "ERROR") {
            window.clearTimeout(timeoutId);
            activeWorker.removeEventListener("message", onMessage);
            reject(new Error(eventPayload.error || "Studio worker failed"));
          }
        };

        activeWorker.addEventListener("message", onMessage);
        const message = { type, payload: { requestId, ...payload } };
        if (payload.imageData instanceof ImageData) {
          activeWorker.postMessage(message, { transfer: [payload.imageData.data.buffer] });
        } else {
          activeWorker.postMessage(message);
        }
      });
    },
    [ensureWorker],
  );

  const renderFromWorker = useCallback(
    async (
      nextAdjustments: StudioEditAdjustments,
      nextComposition: StudioComposition,
      previewMaxSize: number,
    ): Promise<WorkerResult> => {
      const sourceImageData = sourceImageDataRef.current;
      const sourceOriginalSize = sourceOriginalSizeRef.current;

      if (!workerHasSourceRef.current) {
        if (!sourceImageData || !sourceOriginalSize) {
          throw new Error("Studio source image is not loaded");
        }
        const loaded = await callWorker<WorkerResult>(
          "LOAD_IMAGE_DATA",
          {
            imageData: cloneImageData(sourceImageData),
            originalWidth: sourceOriginalSize.width,
            originalHeight: sourceOriginalSize.height,
            adjustments: nextAdjustments,
            composition: nextComposition,
            previewMaxSize,
          },
          "IMAGE_LOADED",
        );
        workerHasSourceRef.current = true;
        return loaded;
      }

      return callWorker<WorkerResult>(
        "RENDER_PREVIEW",
        { adjustments: nextAdjustments, composition: nextComposition, previewMaxSize },
        "PREVIEW_COMPLETE",
      );
    },
    [callWorker],
  );

  const setPreviewBlob = useCallback((blob: Blob) => {
    const url = URL.createObjectURL(blob);
    setPreviewUrl((previous) => {
      if (previous) URL.revokeObjectURL(previous);
      previewUrlRef.current = url;
      return url;
    });
  }, []);

  // ----- Adjustment mutations -----
  const pushHistory = useCallback((prev: StudioEditAdjustments) => {
    setHistory((h) => [...h.slice(-49), prev]);
  }, []);

  const updateAdjustment = useCallback(
    (key: AdjustmentKey, value: number) => {
      setJustSaved(false);
      setAdjustments((prev) => {
        pushHistory(prev);
        return { ...prev, [key]: value };
      });
    },
    [pushHistory],
  );

  const updateGeometry = useCallback(
    (key: "rotation" | "flipHorizontal" | "flipVertical", value: number | boolean) => {
      setJustSaved(false);
      setAdjustments((prev) => {
        pushHistory(prev);
        return { ...prev, [key]: value } as StudioEditAdjustments;
      });
    },
    [pushHistory],
  );

  const undo = useCallback(() => {
    setJustSaved(false);
    setHistory((h) => {
      if (h.length === 0) return h;
      const last = h[h.length - 1];
      setAdjustments(last);
      return h.slice(0, -1);
    });
  }, []);

  const resetAll = useCallback(() => {
    setJustSaved(false);
    setAdjustments((prev) => {
      pushHistory(prev);
      return DEFAULT_STUDIO_ADJUSTMENTS;
    });
  }, [pushHistory]);

  // ----- Asset load -----
  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setIsLoadingAsset(true);
      setError(null);
      setExifRows([]);
      setImageSize(null);
      setFrameExif({});
      setPhotoBitmap((previous) => {
        previous?.close();
        return null;
      });

      try {
        const assetResponse = await client.GET("/api/v1/assets/{id}", {
          params: { path: { id: assetId } },
        });
        const loadedAsset = unwrapData(assetResponse.data, isAsset);
        if (!loadedAsset) throw new Error("Asset response did not include a photo");

        const sidecarResponse = await apiClient.GET("/api/v1/assets/{id}/sidecar", {
          params: { path: { id: assetId } },
        });
        const loadedSidecar = unwrapData(sidecarResponse.data, isSidecarResponse);
        const nextAdjustments = normalizeStudioAdjustments(loadedSidecar?.sidecar.adjustments);
        const nextComposition = normalizeStudioComposition(loadedSidecar?.sidecar);

        const [imageResponse, exifResponse] = await Promise.all([
          fetch(getStudioSourceUrl(loadedAsset, assetId)),
          client
            .GET("/api/v1/assets/{id}/exif", { params: { path: { id: assetId } } })
            .catch(() => null),
        ]);

        if (!imageResponse.ok) {
          throw new Error(`Failed to load Studio source image (${imageResponse.status})`);
        }

        const imageBlob = await imageResponse.blob();
        if (cancelled) return;

        skipNextRenderRef.current = true;
        setAsset(loadedAsset);
        setAdjustments(nextAdjustments);
        compositionRef.current.reset(nextComposition);
        setHistory([]);
        lastSavedSignatureRef.current = JSON.stringify({
          adjustments: nextAdjustments,
          composition: nextComposition,
        });

        let decoded: Awaited<ReturnType<typeof decodeBlobToImageData>>;
        try {
          decoded = await decodeBlobToImageData(imageBlob, 4096);
        } catch (decodeError) {
          throw new Error(
            decodeError instanceof Error
              ? `The exported source image cannot be decoded. ${decodeError.message}`
              : "The exported source image cannot be decoded.",
          );
        }
        if (cancelled) return;

        const originalUrl = URL.createObjectURL(imageBlob);
        setOriginalPreviewUrl((prev) => {
          if (prev) URL.revokeObjectURL(prev);
          originalPreviewUrlRef.current = originalUrl;
          return originalUrl;
        });

        const initialPreviewUrl = URL.createObjectURL(imageBlob);
        setPreviewUrl((prev) => {
          if (prev) URL.revokeObjectURL(prev);
          previewUrlRef.current = initialPreviewUrl;
          return initialPreviewUrl;
        });

        const originalWidth = getAssetDimension(loadedAsset.width, decoded.originalWidth);
        const originalHeight = getAssetDimension(loadedAsset.height, decoded.originalHeight);
        sourceImageDataRef.current = decoded.imageData;
        sourceOriginalSizeRef.current = {
          width: originalWidth,
          height: originalHeight,
        };
        workerHasSourceRef.current = false;

        const render = await callWorker<WorkerResult>(
          "LOAD_IMAGE_DATA",
          {
            imageData: cloneImageData(decoded.imageData),
            originalWidth,
            originalHeight,
            adjustments: nextAdjustments,
            previewMaxSize: 1800,
          },
          "IMAGE_LOADED",
        );
        if (cancelled) return;
        workerHasSourceRef.current = true;

        setPreviewBlob(render.blob);
        setRenderEngine(render.engine);
        const width = render.originalWidth ?? loadedAsset.width ?? render.width;
        const height = render.originalHeight ?? loadedAsset.height ?? render.height;
        setImageSize({ width, height });

        const exif = exifResponse
          ? (unwrapData(exifResponse.data, isExifResponse)?.exif_raw ?? null)
          : null;
        setExifRows(filterExifRows(exif));

        setFrameExif(extractFrameExif(exif));

        // Template previews and adaptive overlay ink need real pixels. The
        // developed preview is the right source: presets should be judged
        // against the photo as edited, not as imported.
        const bitmap = await createImageBitmap(render.blob);
        if (cancelled) {
          bitmap.close();
          return;
        }
        setPhotoBitmap((previous) => {
          previous?.close();
          return bitmap;
        });

        onActivity?.({
          assetId,
          name: loadedAsset.original_filename ?? assetId,
          width: loadedAsset.width ?? width ?? null,
          height: loadedAsset.height ?? height ?? null,
        });
      } catch (loadError) {
        if (cancelled) return;
        const message =
          loadError instanceof Error ? loadError.message : "Failed to load Studio asset";
        setError(message);
        showMessage("error", message);
      } finally {
        if (!cancelled) setIsLoadingAsset(false);
      }
    };

    load().catch(() => {
      // handled via state
    });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [assetId]);

  // ----- Debounced preview render on adjustment change -----
  useEffect(() => {
    if (!asset) return;
    if (skipNextRenderRef.current) {
      skipNextRenderRef.current = false;
      return;
    }

    const generation = ++renderGenerationRef.current;
    const timeoutId = window.setTimeout(() => {
      setIsRendering(true);
      setError(null);
      renderFromWorker(adjustments, currentComposition, 1800)
        .then((result) => {
          if (generation !== renderGenerationRef.current) return;
          setPreviewBlob(result.blob);
          setRenderEngine(result.engine);
          setError(null);
        })
        .catch((renderError) => {
          if (generation !== renderGenerationRef.current) return;
          setError(renderError instanceof Error ? renderError.message : "Failed to render preview");
        })
        .finally(() => {
          if (generation === renderGenerationRef.current) setIsRendering(false);
        });
    }, 80);

    return () => window.clearTimeout(timeoutId);
  }, [asset, adjustments, currentComposition, renderFromWorker, setPreviewBlob]);

  // ----- Cleanup -----
  useEffect(() => {
    return () => {
      workerRef.current?.terminate();
      workerRef.current = null;
      if (savedTimerRef.current) window.clearTimeout(savedTimerRef.current);
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current);
      if (originalPreviewUrlRef.current) URL.revokeObjectURL(originalPreviewUrlRef.current);
    };
  }, []);

  // ----- Save / Export -----
  const handleSave = useCallback(async () => {
    if (!asset?.asset_id) return;
    setIsSaving(true);
    setError(null);
    try {
      const sidecar = createSidecar(asset, adjustments, currentComposition);
      const response = await apiClient.PUT("/api/v1/assets/{id}/sidecar", {
        params: { path: { id: asset.asset_id } },
        body: sidecar,
      });
      if (response.error) throw new Error("Failed to save sidecar");

      lastSavedSignatureRef.current = currentSignature;
      setJustSaved(true);
      if (savedTimerRef.current) window.clearTimeout(savedTimerRef.current);
      savedTimerRef.current = window.setTimeout(() => setJustSaved(false), 2200);
      showMessage("success", t("studio.editor.savedToast", { defaultValue: "Sidecar saved" }));

      onActivity?.({
        assetId: asset.asset_id,
        name: asset.original_filename ?? asset.asset_id,
        width: asset.width ?? imageSize?.width ?? null,
        height: asset.height ?? imageSize?.height ?? null,
      });
    } catch (saveError) {
      const message = saveError instanceof Error ? saveError.message : "Failed to save sidecar";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsSaving(false);
    }
  }, [adjustments, asset, currentComposition, currentSignature, imageSize, onActivity, showMessage, t]);

  const handleExport = useCallback(async () => {
    if (!asset) return;
    const baseName = (asset.original_filename ?? "lumilio-edit").replace(/\.[^.]+$/, "");

    setIsExporting(true);
    try {
      const result = await callWorker<WorkerResult>(
        "EXPORT_IMAGE",
        {
          adjustments,
          composition: currentComposition,
          format: "image/jpeg",
          quality: 0.92,
          maxSize: 8192,
        },
        "EXPORT_COMPLETE",
      );
      const url = URL.createObjectURL(result.blob);
      triggerDownload(url, `${baseName}-lumilio.jpg`);
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (exportError) {
      const message = exportError instanceof Error ? exportError.message : "Failed to export image";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsExporting(false);
    }
  }, [adjustments, asset, currentComposition, callWorker, showMessage]);

  const fileName =
    asset?.original_filename ?? t("studio.editor.loading", { defaultValue: "Loading…" });
  // Geometry, canvas, and layers are all rendered into the preview by the
  // worker, so the viewport presents it as-is. Rotating with CSS here would
  // spin the frame along with the photo.
  const previewAspect = useMemo(() => {
    const quarterTurn = adjustments.rotation === 90 || adjustments.rotation === 270;
    return quarterTurn ? 1 / baseAspect : baseAspect;
  }, [baseAspect, adjustments.rotation]);

  return (
    <div className="flex h-full min-h-0 flex-col" data-screen-label="Studio Editor">
      <TopBar
        fileName={fileName}
        engine={renderEngine}
        dirty={isDirty}
        justSaved={justSaved}
        canUndo={history.length > 0}
        beforeActive={showOriginal}
        isSaving={isSaving}
        isExporting={isExporting}
        onBack={onBack}
        onUndo={undo}
        onResetAll={resetAll}
        onBeforeDown={() => setShowOriginal(true)}
        onBeforeUp={() => setShowOriginal(false)}
        onSave={handleSave}
        onExport={handleExport}
        onToggleDevelopPanel={() => setMobileDevelopOpen((v) => !v)}
      />

      <div className="flex min-h-0 flex-1">
        <AssetPanel
          assetId={asset?.asset_id ?? null}
          fileName={asset?.original_filename ?? "-"}
          sizeText={formatBytes(asset?.file_size)}
          dimensionsText={imageSize ? `${imageSize.width} × ${imageSize.height}` : "-"}
          typeText={asset?.mime_type ?? "-"}
          exifRows={exifRows}
        />

        <Viewport
          previewUrl={previewUrl}
          originalUrl={originalPreviewUrl}
          showOriginal={showOriginal}
          sourceAspect={previewAspect}
          rotation={0}
          flipH={false}
          flipV={false}
          loading={isLoadingAsset || isRendering}
          error={error}
          onDismissError={() => setError(null)}
          fileName={fileName}
        />

        <EditorPanel
          tab={tab}
          onTabChange={setTab}
          adjustments={adjustments}
          onAdjustmentChange={updateAdjustment}
          onGeometryChange={updateGeometry}
          onResetAll={resetAll}
          canvas={composition.canvas}
          layers={composition.layers}
          activeTemplateId={composition.activeTemplateId}
          templatePreviews={composition.templatePreviews}
          exifAvailable={composition.exifAvailable}
          selectedLayerId={composition.selectedLayerId}
          onApplyTemplate={(id) => void composition.applyTemplateById(id)}
          onClearTemplate={composition.clearTemplate}
          onCanvasChange={composition.setCanvas}
          onLayersChange={composition.setLayers}
          onSelectLayer={composition.setSelectedLayerId}
          disabled={!asset || isLoadingAsset}
          mobileOpen={mobileDevelopOpen}
          onMobileClose={() => setMobileDevelopOpen(false)}
        />
      </div>
    </div>
  );
}

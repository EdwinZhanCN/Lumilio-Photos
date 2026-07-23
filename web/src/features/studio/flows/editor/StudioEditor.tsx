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
import { StatusBar } from "./StatusBar";
import { AssetPanel, type AssetExifRow } from "./AssetPanel";
import { Viewport } from "./Viewport";
import { EditorPanel, type EditorTab } from "./EditorPanel";
import { useComposition } from "./useComposition";
import {
  ExportPanel,
  DEFAULT_EXPORT_SETTINGS,
  type ExportFormat,
  type ExportSettings,
} from "./export/ExportPanel";
import { preserveExif } from "../../modules/export/exif";
import { CropOverlay } from "./crop/CropOverlay";
import { TextOverlay } from "./text/TextOverlay";
import { getAspectRatio } from "../../modules/crop/cropMath";
import { displayedFrameSize } from "../../modules/rendering/coordinateSystem";
import { estimateDepthField, disposeDepthPipeline } from "../../modules/depth/depthEstimation";

export type DepthStatus = "idle" | "generating" | "ready" | "error";

type RenderEngineName = "webgl2" | "webgpu";

type PreviewResult = {
  outWidth: number;
  outHeight: number;
  engine?: RenderEngineName;
};

type LoadResult = PreviewResult & {
  originalWidth: number;
  originalHeight: number;
  sourceWidth: number;
  sourceHeight: number;
  snapshot: ImageBitmap;
};

type ExportResult = {
  blob: Blob;
  width: number;
  height: number;
  downscaled?: boolean;
  nativeLongEdge?: number;
};

const EXPORT_EXTENSION: Record<ExportFormat, string> = {
  "image/jpeg": "jpg",
  "image/png": "png",
  "image/webp": "webp",
};

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
  /** Tab to open on entry, e.g. arriving from a Studio Home tool card. */
  initialTab?: EditorTab;
};

const apiClient = client as typeof client & {
  GET: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
  PUT: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
};

// The working source is fetched at up to this long edge — the export ceiling.
// 8192 covers the full resolution of essentially every camera (a 45MP frame is
// 8192×5464); larger sources are guardrailed down in the worker/GPU. Preview
// still renders far smaller, so this only affects export fidelity and memory.
const WORKING_SOURCE_MAX = 8192;

function getStudioSourceUrl(asset: Asset, assetId: string): string {
  return assetUrls.getExportUrl(assetId, {
    format: "jpeg",
    quality: 95,
    maxWidth: WORKING_SOURCE_MAX,
    maxHeight: WORKING_SOURCE_MAX,
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

// ===========================================================================
// StudioEditor
// ===========================================================================
export function StudioEditor({
  assetId,
  onBack,
  onActivity,
  initialTab = "develop",
}: StudioEditorProps): React.JSX.Element {
  const { t } = useI18n();
  const showMessage = useMessage();

  // Worker + canvas refs
  const workerRef = useRef<Worker | null>(null);
  const workerRequestIdRef = useRef(0);
  const canvasElRef = useRef<HTMLCanvasElement | null>(null);
  const originalPreviewUrlRef = useRef<string | null>(null);
  // The untouched original file, fetched lazily on export to copy its EXIF.
  const exifSourceRef = useRef<{ assetId: string; blob: Blob } | null>(null);
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
  const [originalPreviewUrl, setOriginalPreviewUrl] = useState<string | null>(null);
  const [showOriginal, setShowOriginal] = useState(false);
  // Desktop side-rail visibility; mobile uses one bottom sheet at a time.
  const [leftOpen, setLeftOpen] = useState(true);
  const [rightOpen, setRightOpen] = useState(true);
  const [mobileSheet, setMobileSheet] = useState<"none" | "info" | "edit">("none");
  const [imageSize, setImageSize] = useState<{ width: number; height: number } | null>(null);
  // Effective source (post-guardrail) — the pixel space crop rectangles live in.
  const [cropSpace, setCropSpace] = useState<{ width: number; height: number } | null>(null);
  const [cropAspectKey, setCropAspectKey] = useState("free");
  const [cropResetToken, setCropResetToken] = useState(0);
  const [outputAspect, setOutputAspect] = useState(3 / 2);
  const [ready, setReady] = useState(false);
  const [exifRows, setExifRows] = useState<AssetExifRow[]>([]);
  const [isLoadingAsset, setIsLoadingAsset] = useState(false);
  const [isRendering, setIsRendering] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [justSaved, setJustSaved] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const [exportSettings, setExportSettings] = useState<ExportSettings>(DEFAULT_EXPORT_SETTINGS);
  const [renderEngine, setRenderEngine] = useState<RenderEngineName | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  // Bumped when marks reach the worker, to re-render a composition whose logos
  // rasterized after its first paint (logos arrive async, gated on the snapshot).
  const [logosVersion, setLogosVersion] = useState(0);
  // Scene depth for layer occlusion.
  const [depthStatus, setDepthStatus] = useState<DepthStatus>("idle");
  const [depthFeather, setDepthFeather] = useState(0.08);
  const [depthVersion, setDepthVersion] = useState(0);
  // A text layer being dragged on-canvas: hidden from the worker so its live DOM
  // preview is the only copy, avoiding a lagging rasterized duplicate.
  const [hiddenLayerId, setHiddenLayerId] = useState<string | null>(null);

  // Composition
  const [tab, setTab] = useState<EditorTab>(initialTab);
  const [frameExif, setFrameExif] = useState<FrameExif>({});
  const [photoBitmap, setPhotoBitmap] = useState<ImageBitmap | null>(null);

  const sendLogosToWorker = useCallback((logos: Map<string, ImageBitmap>) => {
    if (logos.size === 0) return;
    const worker = workerRef.current;
    if (!worker) return;
    const entries = Array.from(logos.entries());
    worker.postMessage(
      { type: "SET_LOGOS", payload: { requestId: ++workerRequestIdRef.current, logos: entries } },
      entries.map(([, bitmap]) => bitmap),
    );
    setLogosVersion((v) => v + 1);
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

  const originalAspect = useMemo(() => {
    if (imageSize && imageSize.width > 0 && imageSize.height > 0) {
      return imageSize.width / imageSize.height;
    }
    return 3 / 2;
  }, [imageSize]);

  const handleCanvasReady = useCallback((canvas: HTMLCanvasElement) => {
    canvasElRef.current = canvas;
  }, []);

  // ----- Worker plumbing -----
  const callWorker = useCallback(
    <T,>(
      worker: Worker,
      type: "LOAD_IMAGE" | "RENDER" | "EXPORT_IMAGE" | "SNAPSHOT",
      payload: Record<string, unknown>,
      successType: string,
    ): Promise<T> => {
      const requestId = ++workerRequestIdRef.current;
      return new Promise<T>((resolve, reject) => {
        const timeoutId = window.setTimeout(() => {
          worker.removeEventListener("message", onMessage);
          reject(new Error("Studio worker timed out"));
        }, 20000);

        const onMessage = (event: MessageEvent) => {
          const { type: eventType, payload: eventPayload } = event.data || {};
          if (!eventPayload || eventPayload.requestId !== requestId) return;
          if (eventType === successType) {
            window.clearTimeout(timeoutId);
            worker.removeEventListener("message", onMessage);
            resolve(eventPayload as T);
            return;
          }
          if (eventType === "ERROR") {
            window.clearTimeout(timeoutId);
            worker.removeEventListener("message", onMessage);
            reject(new Error(eventPayload.error || "Studio worker failed"));
          }
        };

        worker.addEventListener("message", onMessage);
        worker.postMessage({ type, payload: { requestId, ...payload } });
      });
    },
    [],
  );

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

  // ----- Crop -----
  const cropMode = tab === "crop";

  const handleCropChange = useCallback(
    (crop: StudioEditAdjustments["crop"]) => {
      setJustSaved(false);
      setAdjustments((prev) => {
        if (JSON.stringify(prev.crop) === JSON.stringify(crop)) return prev;
        pushHistory(prev);
        return { ...prev, crop };
      });
    },
    [pushHistory],
  );

  const handleResetCrop = useCallback(() => {
    setCropAspectKey("free");
    setCropResetToken((token) => token + 1);
    handleCropChange(null);
  }, [handleCropChange]);

  // ----- Asset load -----
  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setIsLoadingAsset(true);
      setReady(false);
      setDepthStatus("idle");
      setError(null);
      setExifRows([]);
      setImageSize(null);
      setFrameExif({});
      setPhotoBitmap((previous) => {
        previous?.close();
        return null;
      });

      // A fresh worker per asset: the previous one holds a transferred canvas we
      // can no longer draw to, and its source texture is the wrong photo.
      workerRef.current?.terminate();
      workerRef.current = null;

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

        const originalUrl = URL.createObjectURL(imageBlob);
        setOriginalPreviewUrl((prev) => {
          if (prev) URL.revokeObjectURL(prev);
          originalPreviewUrlRef.current = originalUrl;
          return originalUrl;
        });

        // Spin up the worker and hand it control of the on-screen canvas before
        // asking for the first render — the worker draws straight onto it.
        const canvas = canvasElRef.current;
        if (!canvas) throw new Error("Preview canvas is not mounted");
        const worker = new Worker(
          new URL("../../modules/rendering/studioEdit.worker.ts", import.meta.url),
          { type: "module" },
        );
        workerRef.current = worker;
        const offscreen = canvas.transferControlToOffscreen();
        worker.postMessage(
          {
            type: "INIT_CANVAS",
            payload: { requestId: ++workerRequestIdRef.current, canvas: offscreen },
          },
          [offscreen],
        );

        const loaded = await callWorker<LoadResult>(
          worker,
          "LOAD_IMAGE",
          {
            blob: imageBlob,
            adjustments: nextAdjustments,
            composition: nextComposition,
            previewMaxSize: 1800,
            snapshotMaxSize: 1024,
          },
          "IMAGE_LOADED",
        );
        if (cancelled) {
          loaded.snapshot.close();
          return;
        }

        setReady(true);
        setRenderEngine(loaded.engine);
        setOutputAspect(loaded.outWidth / loaded.outHeight);

        const originalWidth = getAssetDimension(loadedAsset.width, loaded.originalWidth);
        const originalHeight = getAssetDimension(loadedAsset.height, loaded.originalHeight);
        setImageSize({ width: originalWidth, height: originalHeight });
        setCropSpace({ width: loaded.sourceWidth, height: loaded.sourceHeight });

        const exif = exifResponse
          ? (unwrapData(exifResponse.data, isExifResponse)?.exif_raw ?? null)
          : null;
        setExifRows(filterExifRows(exif));
        setFrameExif(extractFrameExif(exif));

        // Template previews and adaptive overlay ink need real pixels: the
        // developed snapshot is judged against the photo as edited on entry.
        setPhotoBitmap((previous) => {
          previous?.close();
          return loaded.snapshot;
        });

        onActivity?.({
          assetId,
          name: loadedAsset.original_filename ?? assetId,
          width: loadedAsset.width ?? originalWidth ?? null,
          height: loadedAsset.height ?? originalHeight ?? null,
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

  // ----- Debounced preview render on adjustment / composition change -----
  useEffect(() => {
    if (!asset || !ready) return;
    if (skipNextRenderRef.current) {
      skipNextRenderRef.current = false;
      return;
    }
    const worker = workerRef.current;
    if (!worker) return;

    const generation = ++renderGenerationRef.current;
    const timeoutId = window.setTimeout(() => {
      setIsRendering(true);
      setError(null);
      // In crop mode the preview shows the whole frame so the crop box can be
      // dragged over all of it; the crop applies once the user leaves the tab.
      const renderAdjustments = cropMode ? { ...adjustments, crop: null } : adjustments;
      // Drop the layer being dragged — the overlay renders it live in the DOM.
      const renderComposition = hiddenLayerId
        ? {
            ...currentComposition,
            layers: currentComposition.layers.filter((layer) => layer.id !== hiddenLayerId),
          }
        : currentComposition;
      callWorker<PreviewResult>(
        worker,
        "RENDER",
        {
          adjustments: renderAdjustments,
          composition: renderComposition,
          previewMaxSize: 1800,
          depthFeather,
        },
        "RENDER_COMPLETE",
      )
        .then((result) => {
          if (generation !== renderGenerationRef.current) return;
          setRenderEngine(result.engine);
          setOutputAspect(result.outWidth / result.outHeight);
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
  }, [
    asset,
    ready,
    adjustments,
    currentComposition,
    logosVersion,
    cropMode,
    depthFeather,
    depthVersion,
    hiddenLayerId,
    callWorker,
  ]);

  // ----- Cleanup -----
  useEffect(() => {
    return () => {
      workerRef.current?.terminate();
      workerRef.current = null;
      if (savedTimerRef.current) window.clearTimeout(savedTimerRef.current);
      if (originalPreviewUrlRef.current) URL.revokeObjectURL(originalPreviewUrlRef.current);
      exifSourceRef.current = null;
      disposeDepthPipeline();
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
  }, [
    adjustments,
    asset,
    currentComposition,
    currentSignature,
    imageSize,
    onActivity,
    showMessage,
    t,
  ]);

  const generateDepth = useCallback(async () => {
    const worker = workerRef.current;
    if (!worker) return;
    setDepthStatus("generating");
    try {
      // Depth must align with the edited image, so estimate on the developed +
      // cropped/rotated photo (no border/layers), not the raw source.
      const snapshot = await callWorker<{ blob: Blob }>(
        worker,
        "SNAPSHOT",
        { adjustments, maxSize: 1024 },
        "SNAPSHOT_COMPLETE",
      );
      const field = await estimateDepthField(snapshot.blob);
      worker.postMessage(
        {
          type: "SET_DEPTH",
          payload: {
            requestId: ++workerRequestIdRef.current,
            depth: field.bitmap,
            feather: depthFeather,
          },
        },
        [field.bitmap],
      );
      setDepthStatus("ready");
      setDepthVersion((version) => version + 1);
    } catch (depthError) {
      setDepthStatus("error");
      showMessage(
        "error",
        depthError instanceof Error
          ? depthError.message
          : t("studio.depth.failed", { defaultValue: "Depth estimation failed" }),
      );
    }
  }, [adjustments, depthFeather, callWorker, showMessage, t]);

  const getExifSource = useCallback(async (id: string): Promise<Blob | null> => {
    if (exifSourceRef.current?.assetId === id) return exifSourceRef.current.blob;
    try {
      const response = await fetch(assetUrls.getOriginalFileUrl(id));
      if (!response.ok) return null;
      const blob = await response.blob();
      exifSourceRef.current = { assetId: id, blob };
      return blob;
    } catch {
      return null;
    }
  }, []);

  const handleExport = useCallback(async () => {
    if (!asset) return;
    const worker = workerRef.current;
    if (!worker) return;
    const baseName = (asset.original_filename ?? "lumilio-edit").replace(/\.[^.]+$/, "");

    setIsExporting(true);
    try {
      const result = await callWorker<ExportResult>(
        worker,
        "EXPORT_IMAGE",
        {
          adjustments,
          composition: currentComposition,
          format: exportSettings.format,
          quality: exportSettings.quality,
          sizeMode: exportSettings.sizeMode,
        },
        "EXPORT_COMPLETE",
      );

      // Copy the original's EXIF onto the re-encoded export (best-effort; PNG and
      // any failure fall through to the raw export).
      let outBlob = result.blob;
      if (exportSettings.format !== "image/png" && asset.asset_id) {
        const originalBlob = await getExifSource(asset.asset_id);
        if (originalBlob) {
          outBlob = await preserveExif(result.blob, originalBlob, {
            format: exportSettings.format,
            width: result.width,
            height: result.height,
          });
        }
      }

      const url = URL.createObjectURL(outBlob);
      triggerDownload(url, `${baseName}-lumilio.${EXPORT_EXTENSION[exportSettings.format]}`);
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setExportOpen(false);

      if (result.downscaled) {
        showMessage(
          "info",
          t("studio.export.downscaledToast", {
            defaultValue: "Export was scaled to {{w}}×{{h}} to stay within limits.",
            w: result.width,
            h: result.height,
          }),
        );
      }
    } catch (exportError) {
      const message = exportError instanceof Error ? exportError.message : "Failed to export image";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsExporting(false);
    }
  }, [
    adjustments,
    asset,
    currentComposition,
    exportSettings,
    callWorker,
    getExifSource,
    showMessage,
    t,
  ]);

  const fileName =
    asset?.original_filename ?? t("studio.editor.loading", { defaultValue: "Loading…" });

  // What the export can actually reach: the smaller of the true source long edge
  // and the working-source cap. The worker may guardrail further on weak GPUs.
  const exportSourceLongEdge = useMemo(() => {
    const longEdge = imageSize ? Math.max(imageSize.width, imageSize.height) : WORKING_SOURCE_MAX;
    return Math.min(WORKING_SOURCE_MAX, longEdge);
  }, [imageSize]);

  // Aspect presets are picked in the DISPLAYED orientation, so "original" and
  // the fit math resolve against the rotated frame.
  const cropAspect = useMemo(() => {
    if (!cropSpace) return null;
    const displayed = displayedFrameSize(cropSpace.width, cropSpace.height, adjustments.rotation);
    return getAspectRatio(cropAspectKey, displayed.width / displayed.height);
  }, [cropSpace, cropAspectKey, adjustments.rotation]);

  const viewportOverlay =
    cropMode && cropSpace ? (
      <CropOverlay
        key={`${assetId}:${cropResetToken}`}
        sourceWidth={cropSpace.width}
        sourceHeight={cropSpace.height}
        rotation={adjustments.rotation}
        flipHorizontal={adjustments.flipHorizontal}
        flipVertical={adjustments.flipVertical}
        aspect={cropAspect}
        initialCrop={adjustments.crop}
        onChange={handleCropChange}
      />
    ) : tab === "text" && ready ? (
      <TextOverlay
        layers={composition.layers}
        selectedLayerId={composition.selectedLayerId}
        onSelectLayer={composition.setSelectedLayerId}
        onLayersChange={composition.setLayers}
        onInteractLayer={setHiddenLayerId}
      />
    ) : undefined;

  return (
    <div className="flex h-full min-h-0 flex-col" data-screen-label="Studio Editor">
      <TopBar
        fileName={fileName}
        canUndo={history.length > 0}
        beforeActive={showOriginal}
        isSaving={isSaving}
        isExporting={isExporting}
        leftOpen={leftOpen}
        rightOpen={rightOpen}
        onBack={onBack}
        onUndo={undo}
        onResetAll={resetAll}
        onBeforeDown={() => setShowOriginal(true)}
        onBeforeUp={() => setShowOriginal(false)}
        onSave={handleSave}
        onExport={() => setExportOpen(true)}
        onToggleLeft={() => setLeftOpen((v) => !v)}
        onToggleRight={() => setRightOpen((v) => !v)}
        onOpenInfo={() => setMobileSheet("info")}
        onOpenEdit={() => setMobileSheet("edit")}
      />

      <div className="flex min-h-0 flex-1">
        <AssetPanel
          assetId={asset?.asset_id ?? null}
          fileName={asset?.original_filename ?? "-"}
          sizeText={formatBytes(asset?.file_size)}
          dimensionsText={imageSize ? `${imageSize.width} × ${imageSize.height}` : "-"}
          typeText={asset?.mime_type ?? "-"}
          exifRows={exifRows}
          open={leftOpen}
          mobileOpen={mobileSheet === "info"}
          onMobileClose={() => setMobileSheet("none")}
        />

        <Viewport
          onCanvasReady={handleCanvasReady}
          canvasKey={assetId}
          originalUrl={originalPreviewUrl}
          showOriginal={showOriginal}
          outputAspect={outputAspect}
          originalAspect={originalAspect}
          ready={ready}
          loading={isLoadingAsset || isRendering}
          error={error}
          onDismissError={() => setError(null)}
          fileName={fileName}
          overlay={viewportOverlay}
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
          cropAspectKey={cropAspectKey}
          onCropAspectChange={setCropAspectKey}
          onResetCrop={handleResetCrop}
          depthStatus={depthStatus}
          depthFeather={depthFeather}
          onGenerateDepth={() => void generateDepth()}
          onDepthFeatherChange={setDepthFeather}
          disabled={!asset || isLoadingAsset}
          open={rightOpen}
          mobileOpen={mobileSheet === "edit"}
          onMobileClose={() => setMobileSheet("none")}
        />
      </div>

      <StatusBar dirty={isDirty} justSaved={justSaved} engine={renderEngine} />

      <ExportPanel
        open={exportOpen}
        settings={exportSettings}
        onChange={setExportSettings}
        sourceLongEdge={exportSourceLongEdge}
        sourceWidth={imageSize?.width ?? 0}
        sourceHeight={imageSize?.height ?? 0}
        isExporting={isExporting}
        onExport={() => void handleExport()}
        onClose={() => setExportOpen(false)}
      />
    </div>
  );
}

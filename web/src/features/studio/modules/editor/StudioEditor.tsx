import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import client from "@/lib/http-commons/client";
import { useI18n } from "@/lib/i18n";
import { useMessage } from "@/features/notifications";
import { useWorker } from "@/contexts/WorkerProvider";
import {
  DEFAULT_PARAMS as BORDER_DEFAULT_PARAMS,
  normalizeParams as normalizeBorderParams,
  isExifBorderMode,
  extractBorderExif,
  hasSufficientExif,
  cameraLabel as borderCameraLabel,
  matchBrandKey,
  brandDisplayName,
  rasterizeBrandLogo,
  type BorderExif,
  type BrandKey,
} from "../tools/border";
import type { BorderExifSummary } from "../tools/border/BorderPanel";
import {
  DEFAULT_STUDIO_ADJUSTMENTS,
  normalizeStudioAdjustments,
  type AssetSidecarResponse,
  type LumilioSidecarV1,
  type StudioEditAdjustments,
} from "./runtime/types";
import type { AdjustmentKey } from "./developConfig";
import { TopBar } from "./TopBar";
import { AssetPanel, type AssetExifRow } from "./AssetPanel";
import { Viewport } from "./Viewport";
import { DevelopPanel } from "../develop/DevelopPanel";

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
  focusBorder?: boolean;
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

function buildBorderExifSummary(exif: BorderExif): BorderExifSummary {
  const matchedKey = matchBrandKey(exif.make, exif.model);
  return {
    available: hasSufficientExif(exif),
    cameraLabel: borderCameraLabel(exif),
    brandText: brandDisplayName(exif.make, matchedKey),
    hasLogo: Boolean(matchedKey),
  };
}

const EMPTY_BORDER_EXIF_SUMMARY: BorderExifSummary = {
  available: false,
  hasLogo: false,
};

function createSidecar(asset: Asset, adjustments: StudioEditAdjustments): LumilioSidecarV1 {
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
    updated_at: new Date().toISOString(),
  };
}

function toPhotometricAdjustments(adjustments: StudioEditAdjustments): StudioEditAdjustments {
  return { ...adjustments, rotation: 0, flipHorizontal: false, flipVertical: false };
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
  focusBorder = false,
}: StudioEditorProps): React.JSX.Element {
  const { t } = useI18n();
  const showMessage = useMessage();
  const workerClient = useWorker();

  // Worker + source refs
  const workerRef = useRef<Worker | null>(null);
  const workerRequestIdRef = useRef(0);
  const workerHasSourceRef = useRef(false);
  const sourceImageDataRef = useRef<ImageData | null>(null);
  const sourceOriginalSizeRef = useRef<{ width: number; height: number } | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const originalPreviewUrlRef = useRef<string | null>(null);
  const borderResultUrlRef = useRef<string | null>(null);
  const lastSavedSignatureRef = useRef(JSON.stringify(DEFAULT_STUDIO_ADJUSTMENTS));
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

  // Border tool
  const [borderParams, setBorderParams] = useState<Record<string, unknown>>(BORDER_DEFAULT_PARAMS);
  const [borderResultUrl, setBorderResultUrl] = useState<string | null>(null);
  const [borderResultFileName, setBorderResultFileName] = useState<string | null>(null);
  const [isApplyingBorder, setIsApplyingBorder] = useState(false);
  // EXIF-driven border data (auto-matched from the asset; not user-editable).
  const borderExifRef = useRef<BorderExif>({});
  const logoBitmapCacheRef = useRef<Map<BrandKey, ImageBitmap | null>>(new Map());
  const [borderExifSummary, setBorderExifSummary] =
    useState<BorderExifSummary>(EMPTY_BORDER_EXIF_SUMMARY);

  const clearLogoCache = useCallback(() => {
    logoBitmapCacheRef.current.forEach((bitmap) => bitmap?.close?.());
    logoBitmapCacheRef.current.clear();
  }, []);

  const getLogoBitmap = useCallback(async (key: BrandKey): Promise<ImageBitmap | null> => {
    const cache = logoBitmapCacheRef.current;
    if (cache.has(key)) return cache.get(key) ?? null;
    const bitmap = await rasterizeBrandLogo(key);
    cache.set(key, bitmap);
    return bitmap;
  }, []);

  const currentSignature = useMemo(() => JSON.stringify(adjustments), [adjustments]);
  const isDirty = currentSignature !== lastSavedSignatureRef.current;
  const photometricAdjustments = useMemo(
    () => toPhotometricAdjustments(adjustments),
    [adjustments],
  );

  const baseAspect = useMemo(() => {
    if (imageSize && imageSize.width > 0 && imageSize.height > 0) {
      return imageSize.width / imageSize.height;
    }
    return 3 / 2;
  }, [imageSize]);

  // ----- Worker plumbing -----
  const ensureWorker = useCallback(() => {
    if (!workerRef.current) {
      workerRef.current = new Worker(new URL("./runtime/studioEdit.worker.ts", import.meta.url), {
        type: "module",
      });
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
            previewMaxSize,
          },
          "IMAGE_LOADED",
        );
        workerHasSourceRef.current = true;
        return loaded;
      }

      return callWorker<WorkerResult>(
        "RENDER_PREVIEW",
        { adjustments: nextAdjustments, previewMaxSize },
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

  const clearBorderResult = useCallback(() => {
    setBorderResultUrl((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      borderResultUrlRef.current = null;
      return null;
    });
    setBorderResultFileName(null);
  }, []);

  // ----- Adjustment mutations -----
  const pushHistory = useCallback((prev: StudioEditAdjustments) => {
    setHistory((h) => [...h.slice(-49), prev]);
  }, []);

  const updateAdjustment = useCallback(
    (key: AdjustmentKey, value: number) => {
      clearBorderResult();
      setJustSaved(false);
      setAdjustments((prev) => {
        pushHistory(prev);
        return { ...prev, [key]: value };
      });
    },
    [clearBorderResult, pushHistory],
  );

  const updateGeometry = useCallback(
    (key: "rotation" | "flipHorizontal" | "flipVertical", value: number | boolean) => {
      clearBorderResult();
      setJustSaved(false);
      setAdjustments((prev) => {
        pushHistory(prev);
        return { ...prev, [key]: value } as StudioEditAdjustments;
      });
    },
    [clearBorderResult, pushHistory],
  );

  const undo = useCallback(() => {
    clearBorderResult();
    setJustSaved(false);
    setHistory((h) => {
      if (h.length === 0) return h;
      const last = h[h.length - 1];
      setAdjustments(last);
      return h.slice(0, -1);
    });
  }, [clearBorderResult]);

  const resetAll = useCallback(() => {
    clearBorderResult();
    setJustSaved(false);
    setAdjustments((prev) => {
      pushHistory(prev);
      return DEFAULT_STUDIO_ADJUSTMENTS;
    });
  }, [clearBorderResult, pushHistory]);

  // ----- Asset load -----
  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setIsLoadingAsset(true);
      setError(null);
      setExifRows([]);
      setImageSize(null);
      clearBorderResult();
      borderExifRef.current = {};
      clearLogoCache();
      setBorderExifSummary(EMPTY_BORDER_EXIF_SUMMARY);

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
        setHistory([]);
        lastSavedSignatureRef.current = JSON.stringify(nextAdjustments);

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
            adjustments: toPhotometricAdjustments(nextAdjustments),
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

        const borderExif = extractBorderExif(exif);
        borderExifRef.current = borderExif;
        setBorderExifSummary(buildBorderExifSummary(borderExif));

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
      renderFromWorker(photometricAdjustments, 1800)
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
  }, [asset, photometricAdjustments, renderFromWorker, setPreviewBlob]);

  // ----- Cleanup -----
  useEffect(() => {
    return () => {
      workerRef.current?.terminate();
      workerRef.current = null;
      if (savedTimerRef.current) window.clearTimeout(savedTimerRef.current);
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current);
      if (originalPreviewUrlRef.current) URL.revokeObjectURL(originalPreviewUrlRef.current);
      if (borderResultUrlRef.current) URL.revokeObjectURL(borderResultUrlRef.current);
      logoBitmapCacheRef.current.forEach((bitmap) => bitmap?.close?.());
      logoBitmapCacheRef.current.clear();
    };
  }, []);

  // ----- Save / Export / Border -----
  const handleSave = useCallback(async () => {
    if (!asset?.asset_id) return;
    setIsSaving(true);
    setError(null);
    try {
      const sidecar = createSidecar(asset, adjustments);
      const response = await apiClient.PUT("/api/v1/assets/{id}/sidecar", {
        params: { path: { id: asset.asset_id } },
        body: sidecar,
      });
      if (response.error) throw new Error("Failed to save sidecar");

      lastSavedSignatureRef.current = JSON.stringify(adjustments);
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
  }, [adjustments, asset, imageSize, onActivity, showMessage, t]);

  const handleExport = useCallback(async () => {
    if (!asset) return;
    const baseName = (asset.original_filename ?? "lumilio-edit").replace(/\.[^.]+$/, "");

    // If a border result is showing, export that baked image directly.
    if (borderResultUrl) {
      triggerDownload(borderResultUrl, borderResultFileName || `${baseName}-border.png`);
      return;
    }

    setIsExporting(true);
    try {
      const result = await callWorker<WorkerResult>(
        "EXPORT_IMAGE",
        { adjustments, format: "image/jpeg", quality: 0.92, maxSize: 8192 },
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
  }, [adjustments, asset, borderResultFileName, borderResultUrl, callWorker, showMessage]);

  const handleApplyBorder = useCallback(async () => {
    if (!asset) return;
    setIsApplyingBorder(true);
    setError(null);
    try {
      // 1) Bake the current develop adjustments into a full-quality image.
      const baseName = (asset.original_filename ?? "lumilio-edit").replace(/\.[^.]+$/, "");
      const developed = await callWorker<WorkerResult>(
        "EXPORT_IMAGE",
        { adjustments, format: "image/png", quality: 0.95, maxSize: 4096 },
        "EXPORT_COMPLETE",
      );
      const developedFile = new File([developed.blob], `${baseName}.png`, {
        type: "image/png",
      });

      // 2) Build params; EXIF-driven modes carry auto-matched EXIF + brand logo.
      const baseParams = normalizeBorderParams(borderParams);
      let applyParams: Record<string, unknown> = { ...baseParams };
      if (isExifBorderMode(baseParams.mode)) {
        const borderExif = borderExifRef.current;
        if (!hasSufficientExif(borderExif)) {
          throw new Error(
            t("studio.tools.border.exifMissing", {
              defaultValue:
                "This style needs camera EXIF (model + at least one of focal length, aperture, shutter, ISO). It's unavailable for this photo.",
            }),
          );
        }
        const matchedKey = matchBrandKey(borderExif.make, borderExif.model);
        const brandText = brandDisplayName(borderExif.make, matchedKey);
        // The Info Strip prefers a rendered logo; Frosted Info always uses text.
        const logo =
          baseParams.mode === "INFO_STRIP" && matchedKey ? await getLogoBitmap(matchedKey) : null;
        applyParams = { ...applyParams, exif: borderExif, brandText, logo };
      }

      // 3) Run the border tool on top of the developed image.
      const result = await workerClient.runTool("border", developedFile, applyParams);

      const url = URL.createObjectURL(result.blob);
      setBorderResultUrl((prev) => {
        if (prev) URL.revokeObjectURL(prev);
        borderResultUrlRef.current = url;
        return url;
      });
      setBorderResultFileName(result.fileName);
      showMessage("success", t("studio.tools.border.done", { defaultValue: "Border applied" }));
    } catch (borderError) {
      const message = borderError instanceof Error ? borderError.message : "Failed to apply border";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsApplyingBorder(false);
    }
  }, [adjustments, asset, borderParams, callWorker, getLogoBitmap, showMessage, t, workerClient]);

  const fileName =
    asset?.original_filename ?? t("studio.editor.loading", { defaultValue: "Loading…" });
  const displaySource = borderResultUrl ?? previewUrl;
  // A border result already has geometry baked in, so present it un-rotated.
  const viewportRotation = borderResultUrl ? 0 : adjustments.rotation;
  const viewportFlipH = borderResultUrl ? false : adjustments.flipHorizontal;
  const viewportFlipV = borderResultUrl ? false : adjustments.flipVertical;
  const viewportAspect = useMemo(() => {
    if (!borderResultUrl) return baseAspect;
    const quarter = adjustments.rotation === 90 || adjustments.rotation === 270;
    return quarter ? 1 / baseAspect : baseAspect;
  }, [borderResultUrl, baseAspect, adjustments.rotation]);

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
          previewUrl={displaySource}
          originalUrl={originalPreviewUrl}
          showOriginal={showOriginal}
          sourceAspect={viewportAspect}
          rotation={viewportRotation}
          flipH={viewportFlipH}
          flipV={viewportFlipV}
          loading={isLoadingAsset || isRendering}
          error={error}
          onDismissError={() => setError(null)}
          fileName={fileName}
        />

        <DevelopPanel
          adjustments={adjustments}
          disabled={!asset || isLoadingAsset}
          focusTools={focusBorder}
          onAdjustmentChange={updateAdjustment}
          onGeometryChange={updateGeometry}
          onResetAll={resetAll}
          borderParams={borderParams}
          onBorderParamsChange={setBorderParams}
          onApplyBorder={handleApplyBorder}
          onClearBorder={clearBorderResult}
          isApplyingBorder={isApplyingBorder}
          hasBorderResult={Boolean(borderResultUrl)}
          borderExifSummary={borderExifSummary}
          mobileOpen={mobileDevelopOpen}
          onMobileClose={() => setMobileDevelopOpen(false)}
        />
      </div>
    </div>
  );
}

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Download,
  FlipHorizontal,
  FlipVertical,
  Image as ImageIcon,
  Loader2,
  RotateCcw,
  RotateCw,
  Save,
  SlidersHorizontal,
  Undo2,
} from "lucide-react";
import PhotoPicker from "@/components/PhotoPicker";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import client from "@/lib/http-commons/client";
import { useI18n } from "@/lib/i18n";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import {
  DEFAULT_STUDIO_ADJUSTMENTS,
  normalizeStudioAdjustments,
  type AssetSidecarResponse,
  type LumilioSidecarV1,
  type StudioEditAdjustments,
} from "@/features/studio/edit-mvp/types";

type ApiResult<T = unknown> = {
  data?: T;
};

type WorkerResult = {
  blob: Blob;
  width: number;
  height: number;
  engine?: "webgpu" | "webgl2" | "wasm-cpu" | "canvas-2d";
  originalWidth?: number;
  originalHeight?: number;
};

type WorkerSuccessType = "IMAGE_LOADED" | "PREVIEW_COMPLETE" | "EXPORT_COMPLETE";

type ExifRow = {
  label: string;
  value: string;
};

const apiClient = client as typeof client & {
  GET: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
  PUT: (url: string, init?: unknown) => Promise<{ data?: unknown; error?: unknown }>;
};

const sliderSections: Array<{
  title: string;
  controls: Array<{
    key: keyof StudioEditAdjustments;
    label: string;
    min: number;
    max: number;
    step: number;
  }>;
}> = [
  {
    title: "Light",
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
    title: "Color",
    controls: [
      { key: "temperature", label: "Temperature", min: -100, max: 100, step: 1 },
      { key: "tint", label: "Tint", min: -100, max: 100, step: 1 },
      { key: "vibrance", label: "Vibrance", min: -100, max: 100, step: 1 },
      { key: "saturation", label: "Saturation", min: -100, max: 100, step: 1 },
    ],
  },
  {
    title: "Detail",
    controls: [
      { key: "clarity", label: "Clarity", min: -100, max: 100, step: 1 },
      { key: "sharpness", label: "Sharpness", min: 0, max: 100, step: 1 },
      { key: "noiseReduction", label: "Noise Reduction", min: 0, max: 100, step: 1 },
    ],
  },
];

function isAsset(value: unknown): value is Asset {
  return Boolean(
    value &&
      typeof value === "object" &&
      typeof (value as Record<string, unknown>).asset_id === "string",
  );
}

function unwrapData<T>(
  response: unknown,
  guard: (value: unknown) => value is T,
): T | undefined {
  if (guard(response)) return response;
  const wrapped = response as ApiResult<unknown> | undefined;
  const data = wrapped?.data;
  if (guard(data)) return data;
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
    return String(value);
  }
  return undefined;
}

function filterExifRows(exif: Record<string, unknown> | null): ExifRow[] {
  if (!exif) return [];

  const fields: Array<[string, string[]]> = [
    ["Camera", ["Model", "CameraModelName", "Make", "EXIF:Model", "Exif.Image.Model"]],
    ["Lens", ["LensModel", "Lens", "EXIF:LensModel", "Exif.Photo.LensModel"]],
    ["ISO", ["ISO", "Exif.Photo.ISOSpeedRatings", "EXIF:ISO"]],
    ["Aperture", ["Aperture", "FNumber", "EXIF:FNumber", "Exif.Photo.FNumber"]],
    ["Shutter", ["ShutterSpeed", "ExposureTime", "EXIF:ExposureTime", "Exif.Photo.ExposureTime"]],
    ["Focal Length", ["FocalLength", "EXIF:FocalLength", "Exif.Photo.FocalLength"]],
    ["Captured", ["DateTimeOriginal", "CreateDate", "EXIF:DateTimeOriginal", "Exif.Photo.DateTimeOriginal"]],
    ["Dimensions", ["ImageSize", "ExifImageWidth", "ExifImageHeight"]],
    ["Orientation", ["Orientation", "EXIF:Orientation", "Exif.Image.Orientation"]],
  ];

  return fields.flatMap(([label, keys]) => {
    const value = getExifValue(exif, keys);
    return value ? [{ label, value }] : [];
  });
}

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

function toPhotometricAdjustments(
  adjustments: StudioEditAdjustments,
): StudioEditAdjustments {
  return {
    ...adjustments,
    rotation: 0,
    flipHorizontal: false,
    flipVertical: false,
  };
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
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
    if (!ctx) {
      throw new Error("Canvas decode context is not available");
    }
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
  return new ImageData(
    new Uint8ClampedArray(imageData.data),
    imageData.width,
    imageData.height,
  );
}

export function StudioEditMvp(): React.JSX.Element {
  const { t } = useI18n();
  const showMessage = useMessage();
  const workerRef = useRef<Worker | null>(null);
  const workerRequestIdRef = useRef(0);
  const lastSavedSignatureRef = useRef(JSON.stringify(DEFAULT_STUDIO_ADJUSTMENTS));
  const skipNextRenderRef = useRef(false);
  const workerHasSourceRef = useRef(false);
  const sourceImageDataRef = useRef<ImageData | null>(null);
  const sourceOriginalSizeRef = useRef<{ width: number; height: number } | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const originalPreviewUrlRef = useRef<string | null>(null);
  const renderGenerationRef = useRef(0);

  const [pickerOpen, setPickerOpen] = useState(true);
  const [assetId, setAssetId] = useState<string | null>(null);
  const [asset, setAsset] = useState<Asset | null>(null);
  const [adjustments, setAdjustments] = useState<StudioEditAdjustments>(
    DEFAULT_STUDIO_ADJUSTMENTS,
  );
  const [history, setHistory] = useState<StudioEditAdjustments[]>([
    DEFAULT_STUDIO_ADJUSTMENTS,
  ]);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [originalPreviewUrl, setOriginalPreviewUrl] = useState<string | null>(null);
  const [showOriginal, setShowOriginal] = useState(false);
  const [imageSize, setImageSize] = useState<{ width: number; height: number } | null>(null);
  const [exifRows, setExifRows] = useState<ExifRow[]>([]);
  const [isLoadingAsset, setIsLoadingAsset] = useState(false);
  const [isRendering, setIsRendering] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [renderEngine, setRenderEngine] = useState<WorkerResult["engine"]>(undefined);
  const [error, setError] = useState<string | null>(null);

  const currentSignature = useMemo(
    () => JSON.stringify(adjustments),
    [adjustments],
  );
  const photometricAdjustments = useMemo(
    () => toPhotometricAdjustments(adjustments),
    [
      adjustments.exposure,
      adjustments.contrast,
      adjustments.highlights,
      adjustments.shadows,
      adjustments.whites,
      adjustments.blacks,
      adjustments.temperature,
      adjustments.tint,
      adjustments.vibrance,
      adjustments.saturation,
      adjustments.clarity,
      adjustments.sharpness,
      adjustments.noiseReduction,
    ],
  );
  const previewTransform = useMemo(() => {
    const transforms = [`rotate(${adjustments.rotation}deg)`];
    if (adjustments.flipHorizontal) transforms.push("scaleX(-1)");
    if (adjustments.flipVertical) transforms.push("scaleY(-1)");
    return transforms.join(" ");
  }, [adjustments.flipHorizontal, adjustments.flipVertical, adjustments.rotation]);
  const isDirty = currentSignature !== lastSavedSignatureRef.current;

  const worker = useCallback(() => {
    if (!workerRef.current) {
      workerRef.current = new Worker(
        new URL("../edit-mvp/studioEdit.worker.ts", import.meta.url),
        { type: "module" },
      );
      workerHasSourceRef.current = false;
    }
    return workerRef.current;
  }, []);

  const callWorker = useCallback(
    <T extends WorkerResult>(
      type: "LOAD_IMAGE" | "LOAD_IMAGE_DATA" | "RENDER_PREVIEW" | "EXPORT_IMAGE",
      payload: Record<string, unknown>,
      successType: WorkerSuccessType,
    ) => {
      const activeWorker = worker();
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
        const message = {
          type,
          payload: {
            requestId,
            ...payload,
          },
        };
        if (payload.imageData instanceof ImageData) {
          activeWorker.postMessage(message, {
            transfer: [payload.imageData.data.buffer],
          });
        } else {
          activeWorker.postMessage(message);
        }
      });
    },
    [worker],
  );

  const setPreviewBlob = useCallback((blob: Blob) => {
    const url = URL.createObjectURL(blob);
    setPreviewUrl((previous) => {
      if (previous) URL.revokeObjectURL(previous);
      previewUrlRef.current = url;
      return url;
    });
  }, []);

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

  const updateAdjustment = useCallback(
    <K extends keyof StudioEditAdjustments>(
      key: K,
      value: StudioEditAdjustments[K],
    ) => {
      setAdjustments((previous) => {
        const next = { ...previous, [key]: value };
        setHistory((items) => [...items.slice(-24), next]);
        return next;
      });
    },
    [],
  );

  const resetAdjustments = useCallback(() => {
    setAdjustments(DEFAULT_STUDIO_ADJUSTMENTS);
    setHistory([DEFAULT_STUDIO_ADJUSTMENTS]);
  }, []);

  const undo = useCallback(() => {
    setHistory((items) => {
      if (items.length <= 1) return items;
      const nextItems = items.slice(0, -1);
      setAdjustments(nextItems[nextItems.length - 1]);
      return nextItems;
    });
  }, []);

  const loadAsset = useCallback(
    async (nextAssetId: string) => {
      setAssetId(nextAssetId);
      setPickerOpen(false);
      setIsLoadingAsset(true);
      setError(null);
      setExifRows([]);
      setImageSize(null);

      try {
        const assetResponse = await client.GET("/api/v1/assets/{id}", {
          params: { path: { id: nextAssetId } },
        });
        const loadedAsset = unwrapData(assetResponse.data, isAsset);
        if (!loadedAsset) {
          throw new Error("Asset response did not include a photo");
        }

        const sidecarResponse = await apiClient.GET(
          "/api/v1/assets/{id}/sidecar",
          { params: { path: { id: nextAssetId } } },
        );
        const loadedSidecar = unwrapData(sidecarResponse.data, isSidecarResponse);
        const nextAdjustments = normalizeStudioAdjustments(
          loadedSidecar?.sidecar.adjustments,
        );

        const [imageResponse, exifResponse] = await Promise.all([
          fetch(assetUrls.getOriginalFileUrl(nextAssetId)),
          client.GET("/api/v1/assets/{id}/exif", {
            params: { path: { id: nextAssetId } },
          }).catch(() => null),
        ]);

        if (!imageResponse.ok) {
          throw new Error(`Failed to load original image (${imageResponse.status})`);
        }

        const imageBlob = await imageResponse.blob();
        const originalUrl = URL.createObjectURL(imageBlob);
        setOriginalPreviewUrl((previous) => {
          if (previous) URL.revokeObjectURL(previous);
          originalPreviewUrlRef.current = originalUrl;
          return originalUrl;
        });
        skipNextRenderRef.current = true;
        setAsset(loadedAsset);
        setAdjustments(nextAdjustments);
        setHistory([nextAdjustments]);
        lastSavedSignatureRef.current = JSON.stringify(nextAdjustments);
        setPreviewUrl((previous) => {
          if (previous) URL.revokeObjectURL(previous);
          const initialPreviewUrl = URL.createObjectURL(imageBlob);
          previewUrlRef.current = initialPreviewUrl;
          return initialPreviewUrl;
        });
        setImageSize(
          loadedAsset.width && loadedAsset.height
            ? { width: loadedAsset.width, height: loadedAsset.height }
            : null,
        );

        const decoded = await decodeBlobToImageData(imageBlob, 4096);
        sourceImageDataRef.current = decoded.imageData;
        sourceOriginalSizeRef.current = {
          width: decoded.originalWidth,
          height: decoded.originalHeight,
        };
        workerHasSourceRef.current = false;
        const render = await callWorker<WorkerResult>(
          "LOAD_IMAGE_DATA",
          {
            imageData: cloneImageData(decoded.imageData),
            originalWidth: decoded.originalWidth,
            originalHeight: decoded.originalHeight,
            adjustments: toPhotometricAdjustments(nextAdjustments),
            previewMaxSize: 1800,
          },
          "IMAGE_LOADED",
        );
        workerHasSourceRef.current = true;

        setPreviewBlob(render.blob);
        setRenderEngine(render.engine);
        setImageSize({
          width: render.originalWidth ?? loadedAsset.width ?? render.width,
          height: render.originalHeight ?? loadedAsset.height ?? render.height,
        });

        const exif = exifResponse
          ? unwrapData(exifResponse.data, isExifResponse)?.exif_raw ?? null
          : null;
        setExifRows(filterExifRows(exif));
      } catch (loadError) {
        const message =
          loadError instanceof Error
            ? loadError.message
            : "Failed to load Studio asset";
        setError(message);
        showMessage("error", message);
      } finally {
        setIsLoadingAsset(false);
      }
    },
    [apiClient, callWorker, setPreviewBlob, showMessage],
  );

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
          const message =
            renderError instanceof Error
              ? renderError.message
              : "Failed to render preview";
          if (generation === renderGenerationRef.current) setError(message);
        })
        .finally(() => {
          if (generation === renderGenerationRef.current) setIsRendering(false);
        });
    }, 80);

    return () => window.clearTimeout(timeoutId);
  }, [asset, photometricAdjustments, renderFromWorker, setPreviewBlob]);

  useEffect(() => {
    return () => {
      workerRef.current?.terminate();
      workerRef.current = null;
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current);
      if (originalPreviewUrlRef.current) URL.revokeObjectURL(originalPreviewUrlRef.current);
    };
  }, []);

  const saveSidecar = useCallback(async () => {
    if (!asset?.asset_id) return;
    setIsSaving(true);
    setError(null);
    try {
      const sidecar = createSidecar(asset, adjustments);
      const response = await apiClient.PUT("/api/v1/assets/{id}/sidecar", {
        params: { path: { id: asset.asset_id } },
        body: sidecar,
      });
      if (response.error) {
        throw new Error("Failed to save sidecar");
      }
      lastSavedSignatureRef.current = JSON.stringify(adjustments);
      showMessage("success", "Sidecar saved");
    } catch (saveError) {
      const message =
        saveError instanceof Error ? saveError.message : "Failed to save sidecar";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsSaving(false);
    }
  }, [adjustments, apiClient, asset, showMessage]);

  const exportImage = useCallback(async () => {
    if (!asset) return;
    setIsExporting(true);
    try {
      const result = await callWorker<WorkerResult>(
        "EXPORT_IMAGE",
        {
          adjustments,
          format: "image/jpeg",
          quality: 0.92,
          maxSize: 8192,
        },
        "EXPORT_COMPLETE",
      );
      const baseName = (asset.original_filename ?? "lumilio-edit").replace(/\.[^.]+$/, "");
      downloadBlob(result.blob, `${baseName}-lumilio.jpg`);
    } catch (exportError) {
      const message =
        exportError instanceof Error ? exportError.message : "Failed to export image";
      setError(message);
      showMessage("error", message);
    } finally {
      setIsExporting(false);
    }
  }, [adjustments, asset, callWorker, showMessage]);

  if (pickerOpen || !assetId) {
    return (
      <div className="h-[calc(100vh-6rem)] overflow-hidden bg-base-100">
        <PhotoPicker
          scopeId="studio:mvp"
          title={t("studio.editMvp.pickPhoto", {
            defaultValue: "Pick a photo to edit",
          })}
          onSelect={(id) => {
            loadAsset(id).catch(() => {
              // handled by loadAsset state
            });
          }}
        />
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-6rem)] flex-col overflow-hidden bg-base-200 text-base-content">
      <div className="flex h-14 items-center justify-between border-b border-base-300 bg-base-100 px-3">
        <div className="flex min-w-0 items-center gap-2">
          <button className="btn btn-sm btn-ghost" onClick={() => setPickerOpen(true)}>
            <ImageIcon className="h-4 w-4" />
            {t("studio.editMvp.changePhoto", { defaultValue: "Change Photo" })}
          </button>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">
              {asset?.original_filename ?? t("studio.editMvp.loading", { defaultValue: "Loading..." })}
            </div>
            <div className="text-xs text-base-content/60">
              {isDirty
                ? t("studio.editMvp.unsaved", { defaultValue: "Unsaved changes" })
                : t("studio.editMvp.saved", { defaultValue: "Saved" })}
              {renderEngine ? ` · ${renderEngine}` : ""}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button className="btn btn-sm btn-ghost" onClick={undo} disabled={history.length <= 1}>
            <Undo2 className="h-4 w-4" />
          </button>
          <button className="btn btn-sm btn-ghost" onClick={resetAdjustments}>
            <RotateCcw className="h-4 w-4" />
            {t("studio.editMvp.reset", { defaultValue: "Reset" })}
          </button>
          <button
            className="btn btn-sm btn-ghost"
            onMouseDown={() => setShowOriginal(true)}
            onMouseUp={() => setShowOriginal(false)}
            onMouseLeave={() => setShowOriginal(false)}
          >
            {t("studio.editMvp.before", { defaultValue: "Before" })}
          </button>
          <button className="btn btn-sm btn-primary" onClick={saveSidecar} disabled={!asset || isSaving}>
            {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            {t("studio.editMvp.save", { defaultValue: "Save" })}
          </button>
          <button className="btn btn-sm" onClick={exportImage} disabled={!asset || isExporting}>
            {isExporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
            {t("studio.editMvp.export", { defaultValue: "Export" })}
          </button>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[260px_minmax(0,1fr)_340px]">
        <aside className="overflow-y-auto border-r border-base-300 bg-base-100 p-4">
          <h2 className="text-sm font-semibold">
            {t("studio.editMvp.asset", { defaultValue: "Asset" })}
          </h2>
          <dl className="mt-3 space-y-2 text-xs">
            <div>
              <dt className="text-base-content/50">File</dt>
              <dd className="break-words">{asset?.original_filename ?? "-"}</dd>
            </div>
            <div>
              <dt className="text-base-content/50">Size</dt>
              <dd>
                {imageSize
                  ? `${imageSize.width} x ${imageSize.height}`
                  : asset?.width && asset?.height
                    ? `${asset.width} x ${asset.height}`
                    : "-"}
              </dd>
            </div>
            <div>
              <dt className="text-base-content/50">Type</dt>
              <dd>{asset?.mime_type ?? "-"}</dd>
            </div>
          </dl>

          <div className="mt-6">
            <h2 className="text-sm font-semibold">EXIF</h2>
            {exifRows.length > 0 ? (
              <dl className="mt-3 space-y-2 text-xs">
                {exifRows.map((row) => (
                  <div key={row.label}>
                    <dt className="text-base-content/50">{row.label}</dt>
                    <dd className="break-words">{row.value}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="mt-3 text-xs text-base-content/60">
                {t("studio.editMvp.noExif", {
                  defaultValue: "No filtered EXIF fields available.",
                })}
              </p>
            )}
          </div>
        </aside>

        <main className="relative min-w-0 overflow-hidden bg-neutral">
          {(isLoadingAsset || isRendering) && (
            <div className="absolute left-3 top-3 z-10 flex items-center gap-2 rounded bg-base-100/90 px-3 py-2 text-xs shadow">
              <Loader2 className="h-4 w-4 animate-spin" />
              {isLoadingAsset
                ? t("studio.editMvp.loadingImage", { defaultValue: "Loading image" })
                : t("studio.editMvp.rendering", { defaultValue: "Rendering" })}
            </div>
          )}

          {error && (
            <div className="absolute left-3 right-3 top-3 z-20 rounded border border-error/30 bg-error/10 px-3 py-2 text-sm text-error">
              {error}
            </div>
          )}

          <div className="flex h-full items-center justify-center p-6">
            {previewUrl ? (
              <img
                className="max-h-full max-w-full object-contain shadow-2xl"
                style={{
                  transform: showOriginal ? undefined : previewTransform,
                  transformOrigin: "center",
                }}
                src={showOriginal && originalPreviewUrl ? originalPreviewUrl : previewUrl}
                alt={asset?.original_filename ?? "Studio preview"}
              />
            ) : (
              <div className="flex flex-col items-center gap-3 text-base-100/70">
                <SlidersHorizontal className="h-12 w-12" />
                <span>{t("studio.editMvp.waiting", { defaultValue: "Waiting for preview" })}</span>
              </div>
            )}
          </div>
        </main>

        <aside className="overflow-y-auto border-l border-base-300 bg-base-100 p-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold">
              {t("studio.editMvp.develop", { defaultValue: "Develop" })}
            </h2>
          </div>

          <div className="mt-4 space-y-5">
            <section>
              <h3 className="mb-3 text-xs font-semibold uppercase text-base-content/60">
                Geometry
              </h3>
              <div className="grid grid-cols-2 gap-2">
                <button
                  className="btn btn-sm"
                  onClick={() =>
                    updateAdjustment(
                      "rotation",
                      (((adjustments.rotation - 90) % 360) + 360) % 360,
                    )
                  }
                >
                  <RotateCcw className="h-4 w-4" />
                  Left
                </button>
                <button
                  className="btn btn-sm"
                  onClick={() =>
                    updateAdjustment("rotation", (adjustments.rotation + 90) % 360)
                  }
                >
                  <RotateCw className="h-4 w-4" />
                  Right
                </button>
                <button
                  className="btn btn-sm"
                  onClick={() =>
                    updateAdjustment("flipHorizontal", !adjustments.flipHorizontal)
                  }
                >
                  <FlipHorizontal className="h-4 w-4" />
                  Flip H
                </button>
                <button
                  className="btn btn-sm"
                  onClick={() =>
                    updateAdjustment("flipVertical", !adjustments.flipVertical)
                  }
                >
                  <FlipVertical className="h-4 w-4" />
                  Flip V
                </button>
              </div>
            </section>

            {sliderSections.map((section) => (
              <section key={section.title}>
                <h3 className="mb-3 text-xs font-semibold uppercase text-base-content/60">
                  {section.title}
                </h3>
                <div className="space-y-3">
                  {section.controls.map((control) => {
                    const value = adjustments[control.key];
                    if (typeof value !== "number") return null;

                    return (
                      <label key={control.key} className="block">
                        <div className="mb-1 flex items-center justify-between gap-2 text-xs">
                          <span>{control.label}</span>
                          <input
                            className="input input-bordered input-xs w-16 text-right"
                            type="number"
                            value={Number(value.toFixed(2))}
                            min={control.min}
                            max={control.max}
                            step={control.step}
                            onChange={(event) =>
                              updateAdjustment(
                                control.key,
                                Number(event.target.value) as never,
                              )
                            }
                          />
                        </div>
                        <input
                          className="range range-xs range-primary"
                          type="range"
                          value={value}
                          min={control.min}
                          max={control.max}
                          step={control.step}
                          onDoubleClick={() => updateAdjustment(control.key, 0 as never)}
                          onChange={(event) =>
                            updateAdjustment(
                              control.key,
                              Number(event.target.value) as never,
                            )
                          }
                        />
                      </label>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        </aside>
      </div>
    </div>
  );
}

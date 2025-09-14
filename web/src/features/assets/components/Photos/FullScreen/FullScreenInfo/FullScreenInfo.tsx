import { useState, useEffect } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { getAssetService } from "@/services/getAssetsService";
import { AxiosError } from "axios";
import { ExifDataDisplay } from "@/features/studio/components/panels/ExifDataDisplay.tsx";
import {
  Camera,
  Aperture,
  Calendar,
  Focus,
  Timer,
  Gauge,
  Copy,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface FullScreenInfoProps {
  asset: Asset;
}

const FullScreenInfo = ({ asset }: FullScreenInfoProps) => {
  const metadata = asset.specific_metadata as any;
  const [detailedExif, setDetailedExif] = useState<any>(null);
  const [isLoadingFile, setIsLoadingFile] = useState(false);
  const showMessage = useMessage();
  const { t } = useI18n();

  const { isExtracting, exifData, extractExifData } = useExtractExifdata();

  async function copyTextToClipboard(value: string): Promise<boolean> {
    try {
      if (
        typeof navigator !== "undefined" &&
        navigator.clipboard?.writeText &&
        window.isSecureContext
      ) {
        await navigator.clipboard.writeText(value);
        return true;
      }
    } catch {
      // continue to fallback
    }
    try {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      textarea.style.pointerEvents = "none";
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      const ok = document.execCommand("copy");
      document.body.removeChild(textarea);
      return ok;
    } catch {
      return false;
    }
  }

  const handleCopy = async (val?: unknown) => {
    const value = val == null ? "" : String(val);
    const ok = await copyTextToClipboard(value);
    showMessage(
      ok ? "success" : "error",
      t(
        ok
          ? "assets.photos.fullscreen.info.copied"
          : "assets.photos.fullscreen.info.copyFailed",
        { defaultValue: ok ? "Copied to clipboard" : "Failed to copy" },
      ),
    );
  };

  const handleExtractExif = async () => {
    try {
      setIsLoadingFile(true);
      setDetailedExif(null); // Clear previous EXIF data

      // Get the original file URL from asset ID
      if (!asset.asset_id) {
        setIsLoadingFile(false);
        showMessage(
          "error",
          t("assets.photos.fullscreen.info.errors.noAssetId", {
            defaultValue: "Asset ID not available",
          }),
        );
        return;
      }

      // Fetch the file using the service method for proper auth handling
      const response = await getAssetService.getOriginalFile(asset.asset_id);
      const blob = response.data;
      const file = new File([blob], asset.original_filename || "image", {
        type: asset.mime_type || "image/jpeg",
      });

      setIsLoadingFile(false);

      // Extract EXIF data
      await extractExifData([file]);
      // Note: exifData will be updated by the hook and handled in useEffect
    } catch (error) {
      setIsLoadingFile(false);

      // Handle specific error cases
      if ((error as AxiosError)?.response?.status === 404) {
        showMessage(
          "error",
          t("assets.photos.fullscreen.info.errors.notFound", {
            defaultValue: "Original file not found on server",
          }),
        );
      } else if ((error as AxiosError)?.response?.status === 401) {
        showMessage(
          "error",
          t("assets.photos.fullscreen.info.errors.unauthorized", {
            defaultValue: "Authentication required to access original file",
          }),
        );
      } else if ((error as AxiosError)?.response?.status === 403) {
        showMessage(
          "error",
          t("assets.photos.fullscreen.info.errors.forbidden", {
            defaultValue: "Access denied to original file",
          }),
        );
      } else {
        showMessage(
          "error",
          t("assets.photos.fullscreen.info.errors.extractFailed", {
            message: (error as Error).message,
            defaultValue: `Failed to extract EXIF: ${(error as Error).message}`,
          }),
        );
      }
    }
  };

  // Watch for exifData changes and update detailedExif
  useEffect(() => {
    if (exifData && Object.keys(exifData).length > 0) {
      // Get the first file's EXIF data (index 0)
      setDetailedExif(exifData[0]);
    }
  }, [exifData]);

  return (
    <div className="absolute top-12 right-0 bottom-0 w-96 bg-base-100/80 p-4 overflow-y-auto z-10 animate-fade-in-x">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-lg font-bold">
          {t("assets.photos.fullscreen.info.title", {
            defaultValue: "Metadata",
          })}
          <span className="badge badge-ghost badge-sm ml-2 align-middle">
            {asset.type || t("common.na", { defaultValue: "N/A" })}
          </span>
        </h2>
        <button
          onClick={handleExtractExif}
          disabled={isExtracting || isLoadingFile}
          className="btn btn-ghost btn-sm"
        >
          {isLoadingFile ? (
            <span className="loading loading-spinner loading-xs"></span>
          ) : isExtracting ? (
            <span className="loading loading-spinner loading-xs"></span>
          ) : (
            t("assets.photos.fullscreen.info.extractComplete", {
              defaultValue: "Extract Complete EXIF",
            })
          )}
        </button>
      </div>

      {/* Basic metadata */}
      <div className="rounded-lg bg-base-100 shadow-sm mb-4">
        <div className="bg-base-300 p-2 rounded-t-lg flex items-center">
          <h3 className="text-sm font-bold text-base-content">
            {t("assets.photos.fullscreen.info.basic", {
              defaultValue: "Basic Information",
            })}
          </h3>
        </div>
        <div className="p-3">
          <div className="flex flex-wrap gap-3">
            {/* Camera */}
            <div className="relative group min-w-[14rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Camera className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Camera className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.camera", {
                      defaultValue: "Camera",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.camera_model ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.camera_model)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* Lens */}
            <div className="relative group min-w-[14rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Focus className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Focus className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.lens", {
                      defaultValue: "Lens",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.lens_model ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.lens_model)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* Aperture */}
            <div className="relative group min-w-[12rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Aperture className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Aperture className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.fNumber", {
                      defaultValue: "F-Number",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.f_number ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.f_number)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* Exposure */}
            <div className="relative group min-w-[12rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Timer className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Timer className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.exposure", {
                      defaultValue: "Exposure",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.exposure_time ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.exposure_time)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* ISO */}
            <div className="relative group min-w-[10rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Gauge className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Gauge className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.iso", {
                      defaultValue: "ISO",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.iso_speed ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.iso_speed)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>

            {/* Taken */}
            <div className="relative group min-w-[14rem] max-w-full flex-1 rounded-xl border border-base-300/60 bg-base-100/80 p-3 overflow-hidden">
              <div className="absolute -right-2 -bottom-2 opacity-10 group-hover:opacity-20 transition-opacity pointer-events-none">
                <Calendar className="w-16 h-16 text-primary" />
              </div>
              <div className="flex items-start gap-2">
                <Calendar className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] uppercase tracking-wide text-base-content/60">
                    {t("assets.photos.fullscreen.info.taken", {
                      defaultValue: "Taken",
                    })}
                  </div>
                  <div className="text-xs font-semibold break-words whitespace-normal">
                    {metadata?.taken_time ||
                      t("common.na", { defaultValue: "N/A" })}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-xs ml-auto"
                  onClick={() => handleCopy(metadata?.taken_time)}
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div className="divide-y divide-base-300/20">
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.filename", {
                defaultValue: "Filename",
              })}
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.original_filename ||
                t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>

          {/* moved to stat tiles: exposure */}

          {/* moved to stat tiles: iso */}

          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.type", {
                defaultValue: "Type",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {asset.type || t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.fileSize", {
                defaultValue: "File Size",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {asset.file_size
                ? `${(asset.file_size / 1024 / 1024).toFixed(2)} MB`
                : t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.dimensions", {
                defaultValue: "Dimensions",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {asset.width && asset.height
                ? `${asset.width} × ${asset.height}`
                : t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.mimeType", {
                defaultValue: "MIME Type",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {asset.mime_type || t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.uploadTime", {
                defaultValue: "Upload Time",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {asset.upload_time
                ? new Date(asset.upload_time).toLocaleString()
                : t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              {t("assets.photos.fullscreen.info.tags", {
                defaultValue: "Tags",
              })}
            </div>
            <div className="text-xs flex-1 break-words whitespace-normal text-right">
              {Array.isArray(asset.tags) && asset.tags.length > 0
                ? asset.tags.map((tag, idx) => (
                    <span
                      key={tag.tag_id || idx}
                      className="inline-block bg-base-200 rounded px-2 py-1 mx-0.5"
                    >
                      {tag.tag_name}
                      {tag.is_ai_generated
                        ? t("assets.photos.fullscreen.info.aiTagSuffix", {
                            defaultValue: " (AI)",
                          })
                        : ""}
                    </span>
                  ))
                : t("common.na", { defaultValue: "N/A" })}
            </div>
          </div>
          {asset.duration && (
            <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
              <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
                {t("assets.photos.fullscreen.info.duration", {
                  defaultValue: "Duration",
                })}
              </div>
              <div className="text-xs flex-1 text-right flex flex-wrap justify-end gap-1">
                {`${Math.floor(asset.duration / 60)}:${(asset.duration % 60).toFixed(0).padStart(2, "0")}`}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Complete EXIF metadata extracted from file - always show for better loading states */}
      <div className="border-t pt-4">
        <ExifDataDisplay
          exifData={detailedExif}
          isLoading={isExtracting || isLoadingFile}
        />
      </div>
    </div>
  );
};

export default FullScreenInfo;

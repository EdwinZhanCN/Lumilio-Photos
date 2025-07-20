import { useState, useRef, useEffect } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata";
import { WorkerClient } from "@/workers/workerClient";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { getAssetService } from "@/services/getAssetsService";
import { AxiosError } from "axios";
import { ExifDataDisplay } from "@/components/Studio/panels/ExifDataDisplay";

interface FullScreenInfoProps {
  asset: Asset;
}

const FullScreenInfo = ({ asset }: FullScreenInfoProps) => {
  const metadata = asset.specific_metadata as any;
  const [detailedExif, setDetailedExif] = useState<any>(null);
  const [isLoadingFile, setIsLoadingFile] = useState(false);
  const workerClientRef = useRef<WorkerClient | null>(null);
  const showMessage = useMessage();

  // Initialize worker client if not already done
  if (!workerClientRef.current) {
    workerClientRef.current = new WorkerClient();
  }

  const { isExtracting, exifData, extractExifData } = useExtractExifdata({
    workerClientRef,
  });

  const handleExtractExif = async () => {
    try {
      setIsLoadingFile(true);
      setDetailedExif(null); // Clear previous EXIF data

      // Get the original file URL from asset ID
      if (!asset.asset_id) {
        setIsLoadingFile(false);
        showMessage("error", "Asset ID not available");
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
        showMessage("error", "Original file not found on server");
      } else if ((error as AxiosError)?.response?.status === 401) {
        showMessage("error", "Authentication required to access original file");
      } else if ((error as AxiosError)?.response?.status === 403) {
        showMessage("error", "Access denied to original file");
      } else {
        showMessage(
          "error",
          `Failed to extract EXIF: ${(error as Error).message}`,
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
    <div className="absolute top-12 right-0 bottom-0 w-96 bg-base-100/80 p-4 overflow-y-auto z-10">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-lg font-bold">Metadata</h2>
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
            "Extract Complete EXIF"
          )}
        </button>
      </div>

      {/* Basic metadata */}
      <div className="rounded-lg bg-base-100 shadow-sm mb-4">
        <div className="bg-base-300 p-2 rounded-t-lg flex items-center">
          <h3 className="text-sm font-bold text-base-content">
            Basic Information
          </h3>
        </div>
        <div className="divide-y divide-base-300/20">
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Filename
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.original_filename || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Camera
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.camera_model || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Lens
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.lens_model || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Exposure
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.exposure_time || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              F-Number
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.f_number || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              ISO
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.iso_speed || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Taken
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {metadata?.taken_time || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Type
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.type || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              File Size
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.file_size
                ? `${(asset.file_size / 1024 / 1024).toFixed(2)} MB`
                : "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Dimensions
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.width && asset.height
                ? `${asset.width} × ${asset.height}`
                : "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              MIME Type
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.mime_type || "N/A"}
            </div>
          </div>
          <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
            <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
              Upload Time
            </div>
            <div className="text-xs flex-1 break-words text-right">
              {asset.upload_time
                ? new Date(asset.upload_time).toLocaleString()
                : "N/A"}
            </div>
          </div>
          {asset.duration && (
            <div className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors">
              <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2">
                Duration
              </div>
              <div className="text-xs flex-1 break-words text-right">
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

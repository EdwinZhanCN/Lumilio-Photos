import { useMemo } from "react";

interface PhotosThumbnailProps {
  asset: Asset;
  openCarousel: (assetId: string) => void;
}

const PhotosThumbnail = ({ asset, openCarousel }: PhotosThumbnailProps) => {
  const thumbnailUrl = useMemo(() => {
    const smallThumbnail = asset.thumbnails?.find((t) => t.size === "small");
    return smallThumbnail?.url || "";
  }, [asset.thumbnails]);

  const containerClasses = [
    "overflow-hidden rounded-lg shadow-md hover:shadow-xl transition-all duration-200",
    "hover:-translate-y-1",
    "break-inside-avoid mb-4",
  ]
    .filter(Boolean)
    .join(" ");

  const imageClasses =
    "w-full h-auto object-cover transition-transform duration-200 hover:scale-105 cursor-pointer";

  return (
    <div key={asset.asset_id} className={containerClasses}>
      {thumbnailUrl ? (
        <img
          src={thumbnailUrl}
          alt={asset.original_filename || "Asset"}
          className={imageClasses}
          loading="lazy"
          onClick={() => openCarousel(asset.asset_id!)}
        />
      ) : (
        <div className="bg-base-300 flex items-center justify-center text-base-content/50 h-40">
          <div className="text-center">
            <div className="text-xs">No Preview</div>
            <div className="text-xs opacity-60">{asset.type}</div>
          </div>
        </div>
      )}

      {/* Asset Info Overlay */}
      <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 hover:opacity-100 transition-opacity duration-200 pointer-events-none">
        <div className="absolute bottom-2 left-2 right-2 text-white text-xs">
          <div className="font-medium truncate">{asset.original_filename}</div>
          {asset.file_size && (
            <div className="opacity-75">
              {(asset.file_size / (1024 * 1024)).toFixed(1)} MB
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default PhotosThumbnail;

import { ViewModeType } from "@/hooks/usePhotosPageState";

interface PhotosThumbnailProps {
  asset: Asset;
  openCarousel: (assetId: string) => void;
  isSelected?: boolean;
  viewMode?: ViewModeType;
}

const PhotosThumbnail = ({
  asset,
  openCarousel,
  isSelected = false,
  viewMode = "masonry",
}: PhotosThumbnailProps) => {
  const thumbnailUrl =
    asset.thumbnails && asset.thumbnails.length > 0
      ? asset.thumbnails[0].URL
      : "";

  const containerClasses = [
    "cursor-pointer overflow-hidden rounded-lg shadow-md hover:shadow-xl transition-all duration-200",
    "hover:-translate-y-1",
    viewMode === "grid" ? "aspect-square" : "break-inside-avoid mb-4",
    isSelected ? "ring-2 ring-primary ring-offset-2" : "",
  ]
    .filter(Boolean)
    .join(" ");

  const imageClasses = [
    "w-full object-cover transition-transform duration-200 hover:scale-105",
    viewMode === "grid" ? "h-full" : "h-auto",
  ].join(" ");

  return (
    <div
      key={asset.assetId}
      className={containerClasses}
      onClick={() => openCarousel(asset.assetId!)}
    >
      {thumbnailUrl ? (
        <img
          src={thumbnailUrl}
          alt={asset.originalFilename || "Asset"}
          className={imageClasses}
          loading="lazy"
        />
      ) : (
        <div
          className={`bg-base-300 flex items-center justify-center text-base-content/50 ${
            viewMode === "grid" ? "h-full" : "h-40"
          }`}
        >
          <div className="text-center">
            <div className="text-xs">No Preview</div>
            <div className="text-xs opacity-60">{asset.type}</div>
          </div>
        </div>
      )}

      {/* Asset Info Overlay */}
      <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 hover:opacity-100 transition-opacity duration-200">
        <div className="absolute bottom-2 left-2 right-2 text-white text-xs">
          <div className="font-medium truncate">{asset.originalFilename}</div>
          {asset.fileSize && (
            <div className="opacity-75">
              {(asset.fileSize / (1024 * 1024)).toFixed(1)} MB
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default PhotosThumbnail;

import { getAssetService } from "@/services/getAssetsService";

interface PhotosThumbnailProps {
  asset: Asset;
  openCarousel: (assetId: string) => void;
}

const PhotosThumbnail = ({ asset, openCarousel }: PhotosThumbnailProps) => {
  const thumbnailUrl = asset.asset_id
    ? getAssetService.getThumbnailUrl(asset.asset_id, "small")
    : undefined;

  const containerClasses = [
    "overflow-hidden shadow-md hover:shadow-xl transition-all duration-200",
    "hover:-translate-y-1",
    "break-inside-avoid mb-1",
    "animate-fade-in",
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
    </div>
  );
};

export default PhotosThumbnail;

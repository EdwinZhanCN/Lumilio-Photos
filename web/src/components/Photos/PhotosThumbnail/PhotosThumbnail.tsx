interface PhotosThumbnailProps {
  asset: Asset;
  openCarousel: (assetId: string) => void;
}

const PhotosThumbnail = ({ asset, openCarousel }: PhotosThumbnailProps) => {
  const thumbnailUrl = asset.thumbnails && asset.thumbnails.length > 0 ? asset.thumbnails[0].URL : '';

  return (
    <div
      key={asset.assetId}
      className="break-inside-avoid mb-4 cursor-pointer overflow-hidden rounded-lg shadow-md hover:shadow-xl transition-shadow"
      onClick={() => openCarousel(asset.assetId!)}
    >
      <img
        src={thumbnailUrl}
        alt={asset.originalFilename || "Asset"}
        className="w-full h-auto object-cover"
        loading="lazy"
      />
    </div>
  );
};

export default PhotosThumbnail;

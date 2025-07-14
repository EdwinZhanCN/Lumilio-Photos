import PhotosThumbnail from "../PhotosThumbnail/PhotosThumbnail";

interface PhotosMasonryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
}

const PhotosMasonry = ({ groupedPhotos, openCarousel }: PhotosMasonryProps) => {
  return (
    <>
      {Object.keys(groupedPhotos).map((groupKey) => (
        <div key={groupKey} className="my-6">
          <h2 className="text-xl font-bold mb-4 text-left">{groupKey}</h2>
          <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-4">
            {groupedPhotos[groupKey].map((asset) => (
              <PhotosThumbnail
                key={asset.assetId}
                asset={asset}
                openCarousel={openCarousel}
              />
            ))}
          </div>
        </div>
      ))}
    </>
  );
};

export default PhotosMasonry;

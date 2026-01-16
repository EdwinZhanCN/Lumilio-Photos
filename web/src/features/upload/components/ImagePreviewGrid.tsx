type ImagePreviewGridProps = {
  previews: (string | null)[];
};

const ImagePreviewGrid = ({ previews }: ImagePreviewGridProps) => {
  if (!previews || previews.length === 0) return null;

  // Filter out null previews to avoid showing empty skeletons
  const validPreviews = previews.filter((url) => url !== null);

  if (validPreviews.length === 0) return null;

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3">
      {validPreviews.map((url, index) => (
        <div
          key={index}
          className="aspect-square rounded-lg overflow-hidden shadow-md hover:shadow-xl transition-shadow duration-200 bg-base-300"
        >
          <img
            src={url}
            alt={`preview ${index + 1}`}
            className="w-full h-full object-cover hover:scale-105 transition-transform duration-200"
            loading="lazy"
          />
        </div>
      ))}
    </div>
  );
};

export default ImagePreviewGrid;

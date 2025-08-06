type ImagePreviewGridProps = {
  previews: (string | null)[];
};

const ImagePreviewGrid = ({ previews }: ImagePreviewGridProps) => {
  if (!previews || previews.length === 0) return null;

  return (
    <div className="grid grid-cols-5 gap-4 mb-6">
      {previews.map((url, index) => (
        <div
          key={index}
          className="aspect-square rounded-lg overflow-hidden shadow-sm"
        >
          {url ? (
            <img
              src={url}
              alt={`preview ${index + 1}`}
              className="w-full h-full object-cover"
            />
          ) : (
            <div className="skeleton h-full w-full"></div>
          )}
        </div>
      ))}
    </div>
  );
};

export default ImagePreviewGrid;

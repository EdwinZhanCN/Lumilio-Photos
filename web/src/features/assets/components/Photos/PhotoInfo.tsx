type PhotoInfoProps = {
  photo: Asset;
  onClose: () => void;
};

export default function PhotoInfo({ photo, onClose }: PhotoInfoProps) {
  return (
    <div className="fixed top-4 right-4 p-4 rounded-lg shadow-lg backdrop-blur backdrop-brightness-90 bg-base-100">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-bold">{photo.original_filename}</h3>
        <button
          onClick={onClose}
          className="text-gray-500 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            strokeWidth={1.5}
            stroke="currentColor"
            className="size-6"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M6 18 18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
      <p className="text-sm mt-2">
        <strong>Date: </strong>
        {photo.specific_metadata?.parse("text")}
      </p>
      <p className="text-sm mt-1">
        <strong>Size: </strong>
        {photo.file_size} bytes
      </p>
      <p className="text-sm mt-1">
        <strong>Tags: </strong>
        {photo.tags?.map((tag) => tag.tag_name).join(", ")}
      </p>
      <p className="text-sm mt-1">
        <strong>Type: </strong>
        {photo.mime_type}
      </p>
      <p className="text-sm mt-1">
        {photo.specific_metadata?.parse("description")}
      </p>
    </div>
  );
}

import ImgStackView from "../ImgStackView/ImgStackView";

export interface Album {
  id: string;
  name: string;
  description?: string;
  imageCount: number;
  coverImages?: string[];
  createdAt: Date;
  updatedAt: Date;
}

interface ImgStackGridProps {
  albums: Album[];
  onAlbumClick?: (album: Album) => void;
  className?: string;
  loading?: boolean;
  emptyMessage?: string;
}

function ImgStackGrid({
  albums,
  onAlbumClick,
  className = "",
  loading = false,
  emptyMessage = "No albums found",
}: ImgStackGridProps) {
  if (loading) {
    return (
      <div
        className={`grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 p-4 ${className}`}
      >
        {Array.from({ length: 12 }).map((_, index) => (
          <div key={index} className="animate-pulse">
            <div className="stack stack-top size-28 mb-2">
              <div className="bg-gray-300 rounded-lg h-24 w-24"></div>
              <div className="bg-gray-200 rounded-lg h-24 w-24"></div>
              <div className="bg-gray-100 rounded-lg h-24 w-24"></div>
            </div>
            <div className="space-y-1">
              <div className="h-4 bg-gray-300 rounded w-3/4"></div>
              <div className="h-3 bg-gray-200 rounded w-1/2"></div>
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (albums.length === 0) {
    return (
      <div
        className={`flex flex-col items-center justify-center p-12 text-center ${className}`}
      >
        <div className="text-6xl mb-4 text-gray-300">📁</div>
        <h3 className="text-lg font-medium text-gray-900 mb-2">No Albums</h3>
        <p className="text-gray-500">{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div
      className={`grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 p-4 ${className}`}
    >
      {albums.map((album) => (
        <div
          key={album.id}
          className="group cursor-pointer transition-transform hover:scale-105"
          onClick={() => onAlbumClick?.(album)}
        >
          <div className="mb-2">
            <ImgStackView />
          </div>
          <div className="space-y-1">
            <h3 className="font-medium text-sm truncate group-hover:text-primary transition-colors">
              {album.name}
            </h3>
            <p className="text-xs text-gray-500">
              {album.imageCount} {album.imageCount === 1 ? "image" : "images"}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

export default ImgStackGrid;

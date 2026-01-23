import ImgStackView from "../ImgStackView/ImgStackView";
import { CheckCircle2, Circle } from "lucide-react";

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
  selectedIds?: string[];
  isSelectionMode?: boolean;
}

function ImgStackGrid({
  albums,
  onAlbumClick,
  className = "",
  loading = false,
  emptyMessage = "No albums found",
  selectedIds = [],
  isSelectionMode = false,
}: ImgStackGridProps) {
  if (loading) {
    return (
      <div
        className={`grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 p-4 min-h-[400px] ${className}`}
      >
        {Array.from({ length: 12 }).map((_, index) => (
          <div key={index} className="animate-pulse flex flex-col items-center">
            <div className="stack stack-top size-28 mb-2">
              <div className="bg-base-300 rounded-lg h-24 w-24"></div>
              <div className="bg-base-200 rounded-lg h-24 w-24"></div>
              <div className="bg-base-100 rounded-lg h-24 w-24"></div>
            </div>
            <div className="space-y-1 w-full flex flex-col items-center">
              <div className="h-4 bg-base-300 rounded w-3/4"></div>
              <div className="h-3 bg-base-200 rounded w-1/2"></div>
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (albums.length === 0) {
    return (
      <div
        className={`flex flex-col items-center justify-center p-12 text-center min-h-[400px] ${className}`}
      >
        <div className="text-6xl mb-4 text-base-300">📁</div>
        <h3 className="text-lg font-medium mb-2">No Albums</h3>
        <p className="text-base-content/60">{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div
      className={`grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-6 p-4 min-h-[400px] ${className}`}
    >
      {albums.map((album) => {
        const isSelected = selectedIds.includes(album.id);
        
        return (
          <div
            key={album.id}
            className={`group relative cursor-pointer transition-all duration-200 flex flex-col items-center text-center p-2 rounded-xl
              ${isSelectionMode ? 'scale-95' : 'hover:scale-105'}
            `}
            onClick={() => onAlbumClick?.(album)}
          >
            {isSelectionMode && (
              <div className="absolute top-1 right-1 z-10">
                {isSelected ? (
                  <CheckCircle2 className="text-primary fill-base-100" size={22} />
                ) : (
                  <Circle className="text-base-content/30" size={22} />
                )}
              </div>
            )}
            
            <div className="mb-3">
              <ImgStackView 
                coverImages={album.coverImages} 
                albumName={album.name} 
                isSelected={isSelected}
              />
            </div>
            
            <div className="space-y-1 w-full px-1">
              <h3 className={`font-semibold text-sm truncate transition-colors
                ${isSelected ? 'text-primary' : 'group-hover:text-primary'}
              `}>
                {album.name}
              </h3>
              <p className="text-xs text-base-content/50 font-medium">
                {album.imageCount} {album.imageCount === 1 ? "item" : "items"}
              </p>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export default ImgStackGrid;

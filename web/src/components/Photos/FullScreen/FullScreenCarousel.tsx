import React, { useState, useMemo } from "react";
import {
  XMarkIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  InformationCircleIcon,
  ShareIcon,
  HeartIcon,
  TrashIcon,
  ArrowsPointingOutIcon,
  EllipsisHorizontalIcon,
} from "@heroicons/react/24/outline";

// --- 1. Type Definitions (as provided) ---
// It's good practice to keep these in a central types file (e.g., src/types/models.ts)

interface Asset {
  assetId?: string;
  uploadTime?: string;
  originalFilename?: string;
  fileSize?: number;
  tags?: AssetTag[];
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  thumbnails?: AssetThumbnail[];
  description?: string;
  specificMetadata?: Record<string, any>; // Using Record<string, any> for flexible JSON
}

interface AssetTag {
  tagId?: number;
  tagName?: string;
}

interface AssetThumbnail {
  size?: "small" | "medium" | "large";
  storagePath?: string;
}

// --- 2. Helper Component for Displaying Metadata ---
// This component dynamically renders any key-value pairs from the specificMetadata object.

const MetadataDisplay: React.FC<{ metadata: Record<string, any> }> = ({
  metadata,
}) => {
  const entries = Object.entries(metadata);

  if (entries.length === 0) {
    return <p className="text-gray-400">No specific metadata available.</p>;
  }

  return (
    <div className="grid grid-cols-3 gap-x-4 gap-y-2 text-sm">
      {entries.map(([key, value]) => (
        <React.Fragment key={key}>
          <dt className="font-semibold text-gray-400 col-span-1 truncate">
            {key}
          </dt>
          <dd className="text-gray-200 col-span-2 truncate">
            {typeof value === "object" ? JSON.stringify(value) : String(value)}
          </dd>
        </React.Fragment>
      ))}
    </div>
  );
};

// --- 3. Improved PhotoInfo Popover Sheet ---
// This component now acts as the side sheet to show all details.

interface PhotoInfoProps {
  asset: Asset;
  onClose: () => void;
}

const PhotoInfo: React.FC<PhotoInfoProps> = ({ asset, onClose }) => {
  return (
    <div className="absolute top-0 right-0 h-full w-96 bg-gray-900 bg-opacity-80 backdrop-blur-md p-6 text-white shadow-2xl overflow-y-auto transition-transform transform translate-x-0">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-bold">Details</h2>
        <button
          onClick={onClose}
          className="p-1 rounded-full hover:bg-white/10"
        >
          <XMarkIcon className="h-6 w-6" />
        </button>
      </div>
      <dl className="space-y-4">
        <div>
          <dt className="font-semibold text-gray-400">Filename</dt>
          <dd className="text-gray-200">{asset.originalFilename || "N/A"}</dd>
        </div>
        <div>
          <dt className="font-semibold text-gray-400">Description</dt>
          <dd className="text-gray-200">
            {asset.description || "No description."}
          </dd>
        </div>
        <div>
          <dt className="font-semibold text-gray-400">File Size</dt>
          <dd className="text-gray-200">
            {asset.fileSize
              ? `${(asset.fileSize / 1024 / 1024).toFixed(2)} MB`
              : "N/A"}
          </dd>
        </div>
        <div>
          <dt className="font-semibold text-gray-400">Upload Date</dt>
          <dd className="text-gray-200">
            {asset.uploadTime
              ? new Date(asset.uploadTime).toLocaleString()
              : "N/A"}
          </dd>
        </div>
        <div className="border-t border-white/20 pt-4">
          <dt className="font-semibold text-gray-400 mb-2">
            Specific Metadata
          </dt>
          {asset.specificMetadata ? (
            <MetadataDisplay metadata={asset.specificMetadata} />
          ) : (
            <p className="text-gray-400">No specific metadata available.</p>
          )}
        </div>
      </dl>
    </div>
  );
};

// --- 4. Main FullScreenCarousel Component ---

interface FullScreenCarouselProps {
  photos: Asset[];
  initialSlide: number;
  onClose: () => void; // A callback to close the carousel
  onNavigate: (newIndex: number) => void; // Callback to update the URL on navigation
}

export default function FullScreenCarousel({
  photos,
  initialSlide,
  onClose,
  onNavigate,
}: FullScreenCarouselProps) {
  const [currentSlide, setCurrentSlide] = useState(initialSlide);
  const [isInfoOpen, setIsInfoOpen] = useState(false);
  const [areControlsVisible, setAreControlsVisible] = useState(true);

  const totalSlides = photos.length;

  const currentAsset = useMemo(
    () => photos[currentSlide],
    [photos, currentSlide],
  );

  const handleNavigation = (newIndex: number) => {
    setCurrentSlide(newIndex);
    onNavigate(newIndex); // Notify parent about the change for URL update
  };

  const handlePrev = () => {
    const newIndex = currentSlide === 0 ? totalSlides - 1 : currentSlide - 1;
    handleNavigation(newIndex);
  };

  const handleNext = () => {
    const newIndex = currentSlide === totalSlides - 1 ? 0 : currentSlide + 1;
    handleNavigation(newIndex);
  };

  // Get the highest quality thumbnail available for display
  const getDisplayUrl = (asset?: Asset) => {
    if (!asset) return "https://placehold.co/1920x1080/000/fff?text=Loading...";
    const large = asset.thumbnails?.find(
      (t) => t.size === "large",
    )?.storagePath;
    const medium = asset.thumbnails?.find(
      (t) => t.size === "medium",
    )?.storagePath;
    return (
      large ||
      medium ||
      "https://placehold.co/1920x1080/000/fff?text=No+Preview"
    );
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-90 overflow-hidden">
      {/* Close Button (top right) */}
      <button
        onClick={onClose}
        className="absolute top-4 right-4 z-50 p-2 text-white bg-black/30 rounded-full hover:bg-black/50 transition-colors"
      >
        <XMarkIcon className="h-7 w-7" />
      </button>

      {/* Backdrop for toggling controls and closing info panel */}
      <div
        className="absolute inset-0"
        onClick={() => {
          if (isInfoOpen) setIsInfoOpen(false);
          else setAreControlsVisible(!areControlsVisible);
        }}
      />

      {/* Main Image */}
      <div className="relative w-full h-full flex items-center justify-center">
        <img
          key={currentAsset?.assetId} // Add key to force re-render on slide change for smooth transitions
          src={getDisplayUrl(currentAsset)}
          alt={currentAsset?.description || "Full screen asset"}
          className="max-w-full max-h-full object-contain animate-fade-in"
        />
      </div>

      {/* Navigation Arrows */}
      <div
        className={`absolute inset-0 flex items-center justify-between px-4 transition-opacity duration-300 ${areControlsVisible ? "opacity-100" : "opacity-0 pointer-events-none"}`}
      >
        <button
          onClick={handlePrev}
          className="btn-circle p-4 cursor-pointer bg-black/30 backdrop-blur-sm hover:bg-black/50"
        >
          <ChevronLeftIcon className="size-6 text-white" />
        </button>
        <button
          onClick={handleNext}
          className="btn-circle p-4 cursor-pointer bg-black/30 backdrop-blur-sm hover:bg-black/50"
        >
          <ChevronRightIcon className="size-6 text-white" />
        </button>
      </div>

      {/* Toolbar */}
      <div
        className={`absolute bottom-5 flex gap-4 items-center bg-gray-800/40 backdrop-blur-sm p-3 rounded-full transition-opacity duration-300 ${areControlsVisible ? "opacity-100" : "opacity-0 pointer-events-none"}`}
      >
        {/* Placeholder Buttons */}
        <button className="p-2 text-white hover:text-blue-400">
          <ShareIcon className="size-6" />
        </button>
        <button className="p-2 text-white hover:text-pink-400">
          <HeartIcon className="size-6" />
        </button>

        {/* Info Button */}
        <button
          onClick={() => setIsInfoOpen(!isInfoOpen)}
          className={`p-2 rounded-full transition-colors ${isInfoOpen ? "bg-blue-500 text-white" : "text-white hover:bg-white/20"}`}
        >
          <InformationCircleIcon className="size-6" />
        </button>

        <div className="text-white text-sm tabular-nums">
          {currentSlide + 1} / {totalSlides}
        </div>

        <button className="p-2 text-white hover:text-red-500">
          <TrashIcon className="size-6" />
        </button>
        <button className="p-2 text-white hover:text-gray-300">
          <EllipsisHorizontalIcon className="size-6" />
        </button>
      </div>

      {/* Info Panel */}
      {isInfoOpen && currentAsset && (
        <PhotoInfo asset={currentAsset} onClose={() => setIsInfoOpen(false)} />
      )}
    </div>
  );
}

// Example CSS for fade-in animation (in your global CSS file)
/*
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
.animate-fade-in {
  animation: fadeIn 0.3s ease-in-out;
}
*/

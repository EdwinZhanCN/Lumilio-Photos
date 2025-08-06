import React from 'react';

interface PhotosLoadingSkeletonProps {
  count?: number;
}

const PhotosLoadingSkeleton: React.FC<PhotosLoadingSkeletonProps> = ({ count = 12 }) => {
  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      {/* Toolbar Skeleton */}
      <div className="flex gap-2 items-center mb-4">
        <div className="skeleton h-8 w-20"></div>
        <div className="skeleton h-8 w-8 rounded-full"></div>
      </div>

      {/* Group Header Skeleton */}
      <div className="my-6">
        <div className="skeleton h-6 w-32 mb-4"></div>

        {/* Masonry Grid Skeleton */}
        <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-4">
          {Array.from({ length: count }).map((_, index) => (
            <div
              key={index}
              className="break-inside-avoid mb-4 overflow-hidden rounded-lg shadow-md"
            >
              <div
                className="skeleton w-full"
                style={{
                  height: `${Math.floor(Math.random() * 200) + 150}px`
                }}
              ></div>
            </div>
          ))}
        </div>
      </div>

      {/* Load More Skeleton */}
      <div className="text-center p-4">
        <div className="skeleton h-4 w-32 mx-auto"></div>
      </div>
    </div>
  );
};

export default PhotosLoadingSkeleton;

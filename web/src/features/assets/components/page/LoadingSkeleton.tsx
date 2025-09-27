import React from "react";

interface PhotosLoadingSkeletonProps {
  count?: number;
}

const PhotosLoadingSkeleton: React.FC<PhotosLoadingSkeletonProps> = () => {
  return (
    <div className="flex p-4 w-full max-w-screen-lg mx-auto justify-center">
      <span className="loading loading-bars loading-xl"></span>
    </div>
  );
};

export default PhotosLoadingSkeleton;

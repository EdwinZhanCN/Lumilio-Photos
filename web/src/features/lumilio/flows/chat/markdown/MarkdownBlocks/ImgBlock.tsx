import React, { useState } from "react";

type ImgBlockProps = {
  src?: string;
  alt?: string;
  title?: string;
  className?: string;
  [key: string]: any;
};

export const Img: React.FC<ImgBlockProps> = ({
  src,
  alt = "",
  title,
  className = "",
  ...props
}) => {
  const [isLoading, setIsLoading] = useState(true);
  const [hasError, setHasError] = useState(false);

  const handleLoad = () => {
    setIsLoading(false);
  };

  const handleError = () => {
    setIsLoading(false);
    setHasError(true);
  };

  if (!src) {
    return (
      <div className="flex items-center justify-center h-32 bg-base-200 rounded-lg border-2 border-dashed border-base-300">
        <span className="text-base-content/50">No image source</span>
      </div>
    );
  }

  if (hasError) {
    return (
      <div className="flex flex-col items-center justify-center h-32 bg-base-200 rounded-lg border border-base-300">
        <svg
          className="w-8 h-8 text-base-content/40 mb-2"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
        <span className="text-sm text-base-content/50">Failed to load image</span>
        {alt && <span className="text-xs text-base-content/40 mt-1">{alt}</span>}
      </div>
    );
  }

  return (
    <div className="relative my-4">
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-base-200 rounded-lg animate-pulse">
          <div className="w-8 h-8 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
        </div>
      )}
      <img
        src={src}
        alt={alt}
        title={title}
        className={`max-w-full h-auto rounded-lg shadow-md transition-opacity duration-300 ${
          isLoading ? "opacity-0" : "opacity-100"
        } ${className}`}
        onLoad={handleLoad}
        onError={handleError}
        loading="lazy"
        {...props}
      />
      {alt && !isLoading && (
        <p className="text-sm text-base-content/60 text-center mt-2 italic">{alt}</p>
      )}
    </div>
  );
};

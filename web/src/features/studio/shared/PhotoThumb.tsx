import React, { useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";

type PhotoThumbProps = {
  assetId?: string | null;
  size?: "small" | "medium" | "large";
  alt?: string;
  className?: string;
  rounded?: string;
};

/**
 * Thumbnail tile backed by the asset thumbnail endpoint, with a graceful
 * gradient fallback when there is no asset id or the image fails to load.
 */
export function PhotoThumb({
  assetId,
  size = "small",
  alt,
  className = "",
  rounded = "rounded-md",
}: PhotoThumbProps): React.JSX.Element {
  const [failed, setFailed] = useState(false);
  const url = assetId ? assetUrls.getThumbnailUrl(assetId, size) : null;

  if (!url || failed) {
    return (
      <div
        className={`bg-base-300 ${rounded} ${className}`}
        aria-hidden="true"
      />
    );
  }

  return (
    <img
      src={url}
      alt={alt ?? ""}
      loading="lazy"
      decoding="async"
      onError={() => setFailed(true)}
      className={`object-cover ${rounded} ${className}`}
    />
  );
}

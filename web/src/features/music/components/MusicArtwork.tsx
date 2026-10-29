import { useState } from "react";
import { Disc3 } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";

type MusicArtworkProps = {
  assetId?: string;
  alt: string;
  size?: "sm" | "md" | "lg" | "cover";
};

const sizeClasses = {
  cover: "aspect-square w-full rounded-xl",
  sm: "size-12 rounded-lg",
  md: "size-24 rounded-xl",
  lg: "size-40 rounded-2xl",
} as const;

export default function MusicArtwork({ assetId, alt, size = "md" }: MusicArtworkProps) {
  const [failedAsset, setFailedAsset] = useState<string>();
  return assetId && failedAsset !== assetId ? (
    <img
      src={assetUrls.getThumbnailUrl(assetId, size === "sm" ? "small" : "medium")}
      loading="lazy"
      onError={() => setFailedAsset(assetId)}
      alt={alt}
      className={`${sizeClasses[size]} bg-base-300 object-cover shadow-sm`}
    />
  ) : (
    <div
      className={`${sizeClasses[size]} flex items-center justify-center bg-base-200 text-base-content/30`}
      role="img"
      aria-label={alt}
    >
      <Disc3
        className={
          size === "lg" || size === "cover" ? "size-16" : size === "md" ? "size-10" : "size-5"
        }
        strokeWidth={1.5}
      />
    </div>
  );
}

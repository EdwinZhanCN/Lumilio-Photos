import type { ReactNode } from "react";
import { Music, Play } from "lucide-react";
import { LoadMoreButton } from "@/components/collection";
import { useI18n } from "@/lib/i18n.tsx";
import { shareUrls } from "../../model/shareUrls";
import type { components } from "@/lib/http-commons/schema.d.ts";

type PublicAssetDTO = components["schemas"]["dto.PublicAssetDTO"];

export interface PublicShareGridProps {
  token: string;
  assets: PublicAssetDTO[];
  onOpen: (assetId: string) => void;
  onLoadMore: () => void;
  hasMore: boolean;
  isLoadingMore: boolean;
}

/**
 * Minimal, dependency-light thumbnail grid for the public share viewer. It
 * intentionally does not reuse JustifiedGallery/SquareGallery — those are
 * built around the full authenticated Asset/BrowseItem types (stacks,
 * selection, media-token thumbnail URLs) that a public share never receives.
 */
export function PublicShareGrid({
  token,
  assets,
  onOpen,
  onLoadMore,
  hasMore,
  isLoadingMore,
}: PublicShareGridProps): ReactNode {
  const { t } = useI18n();

  if (assets.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-24 text-center text-base-content/60">
        <p className="text-sm">{t("share.public.grid.empty", "This share has no items.")}</p>
      </div>
    );
  }

  return (
    <div className="p-2 sm:p-3">
      <div className="grid grid-cols-2 gap-1 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {assets.map((asset) => {
          if (!asset.asset_id) return null;
          const isVideo = asset.type === "VIDEO";
          const isAudio = asset.type === "AUDIO";
          return (
            <button
              key={asset.asset_id}
              type="button"
              className="group relative aspect-square overflow-hidden rounded-lg bg-base-200"
              onClick={() => onOpen(asset.asset_id!)}
            >
              {isAudio ? (
                <div className="flex h-full w-full items-center justify-center">
                  <Music className="size-8 text-base-content/40" />
                </div>
              ) : (
                <img
                  src={shareUrls.getThumbnailUrl(token, asset.asset_id, "medium")}
                  loading="lazy"
                  alt=""
                  className="h-full w-full object-cover transition-transform group-hover:scale-105"
                />
              )}
              {isVideo && (
                <span className="absolute bottom-1.5 right-1.5 rounded-full bg-black/55 p-1 text-white">
                  <Play className="size-3.5 fill-current" />
                </span>
              )}
            </button>
          );
        })}
      </div>

      {hasMore && <LoadMoreButton onClick={onLoadMore} loading={isLoadingMore} />}
    </div>
  );
}

export default PublicShareGrid;

import { useMemo, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Images } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { client } from "@/lib/http-commons/client";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n";
import { getViewerTimeZone } from "../../model/assetGroups";
import {
  browseGroupsFromSearchResultsPage,
  flattenBrowseGroupsToAssets,
} from "../../model/browseItems";
import { classifySearchError, throwSearchError } from "../../model/visualSearch";
import MediaThumbnail from "../browse/gallery/media/MediaThumbnail";

const PREVIEW_LIMIT = 12;

type AssetSimilarRailProps = {
  open: boolean;
  queryAssetId?: string;
  carouselAssets: Asset[];
  onNavigate: (assetId: string) => void;
  onClose: () => void;
};

export function AssetSimilarRail({
  open,
  queryAssetId,
  carouselAssets,
  onNavigate,
  onClose,
}: AssetSimilarRailProps) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { capabilities } = useCapabilities();
  const imageEmbed = capabilities?.ml.tasks.clipImageEmbed ?? { enabled: false, available: false };
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const query = useQuery({
    queryKey: ["post", "/api/v1/assets/search", { similar: queryAssetId, preview: true }],
    queryFn: async ({ signal }) => {
      const { data, error, response } = await client.POST("/api/v1/assets/search", {
        body: {
          similar_to_asset_id: queryAssetId,
          pagination: { limit: PREVIEW_LIMIT, offset: 0 },
          top_results_limit: PREVIEW_LIMIT,
          viewer_timezone: viewerTimeZone,
        },
        signal,
      });
      if (error) throwSearchError(error, response.status);
      return data;
    },
    enabled: open && Boolean(queryAssetId),
  });

  if (!open || !queryAssetId) return null;

  const hits = flattenBrowseGroupsToAssets(
    browseGroupsFromSearchResultsPage({ resultItems: query.data?.result_items }),
  );
  const errorKind = query.error ? classifySearchError(query.error) : null;
  const seeAllHref = `/assets?similar=${encodeURIComponent(queryAssetId)}`;
  const carouselIds = new Set(carouselAssets.map((asset) => asset.asset_id).filter(Boolean));

  const handleHitClick = (hitId: string) => {
    if (carouselIds.has(hitId)) {
      onNavigate(hitId);
      return;
    }
    onClose();
    void navigate(`/assets/${hitId}?similar=${encodeURIComponent(queryAssetId)}`);
  };

  let body: ReactNode;
  if (query.isLoading) {
    body = (
      <p className="px-2 text-sm text-white/60">
        {t("assets.mediaViewer.similarLoading", "Looking for similar media…")}
      </p>
    );
  } else if (!imageEmbed.enabled && errorKind === "unavailable") {
    body = (
      <p className="px-2 text-sm text-white/70">
        {t("assets.mediaViewer.similarDisabled", "Image Semantic Analysis is off.")}
      </p>
    );
  } else if (errorKind === "embedding_missing") {
    body = (
      <p className="px-2 text-sm text-white/70">
        {t(
          "assets.mediaViewer.similarMissingEmbedding",
          "This photo has no Image Semantic Analysis embedding yet.",
        )}
      </p>
    );
  } else if (errorKind === "unavailable") {
    body = (
      <p className="px-2 text-sm text-white/70">
        {t(
          "assets.mediaViewer.similarUnavailable",
          "Image Semantic Analysis is currently unavailable.",
        )}
      </p>
    );
  } else if (query.isError) {
    body = (
      <p className="px-2 text-sm text-white/70">
        {t("assets.mediaViewer.similarError", "Could not load similar media.")}
      </p>
    );
  } else if (hits.length === 0) {
    body = (
      <p className="px-2 text-sm text-white/70">
        {t("assets.mediaViewer.similarEmpty", "Nothing similar enough")}
      </p>
    );
  } else {
    body = (
      <div className="flex gap-2 overflow-x-auto px-1 pb-1">
        {hits.map((asset) => {
          const hitId = asset.asset_id;
          if (!hitId) return null;
          return (
            <button
              key={hitId}
              type="button"
              className="size-16 shrink-0 overflow-hidden rounded-lg"
              onClick={() => handleHitClick(hitId)}
              aria-label={asset.original_filename ?? hitId}
            >
              <MediaThumbnail
                asset={asset}
                thumbnailUrl={assetUrls.getThumbnailUrl(hitId, "small")}
              />
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <aside className="absolute inset-x-0 bottom-6 z-20 mx-auto w-[calc(100vw-3rem)] max-w-5xl rounded-2xl border border-white/10 bg-zinc-950/78 p-3 text-white shadow-2xl backdrop-blur-2xl">
      <div className="mb-2 flex items-center justify-between gap-3 px-1">
        <div className="flex items-center gap-2">
          <Images className="size-4 text-white/70" />
          <h2 className="text-sm font-semibold tracking-wide">
            {t("assets.mediaViewer.similar", "Similar")}
          </h2>
        </div>
        <Link to={seeAllHref} className="btn btn-ghost btn-xs text-white/80" onClick={onClose}>
          {t("assets.mediaViewer.similarSeeAll", "See all")}
        </Link>
      </div>
      {body}
    </aside>
  );
}

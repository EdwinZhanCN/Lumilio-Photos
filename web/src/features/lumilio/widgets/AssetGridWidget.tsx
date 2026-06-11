import { useCallback, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Images, X } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { useI18n } from "@/lib/i18n.tsx";
import type { ApiResult, AgentRefAssetsDTO } from "../types";
import type { WidgetProps } from "./types";
import { useWidgetAssetsInfinite, useWidgetAssetsPreview } from "./useWidgetAssets";

const INLINE_PREVIEW_COUNT = 8;
const PAGE_SIZE = 60;

/** The asset_grid widget: hydrates its set from the ref/pin APIs and renders
 * a photo grid. Inline (chat) shows a compact preview with a view-all modal;
 * board fills its grid cell with a scrollable grid. */
export function AssetGridWidget({ source, variant, count, title }: WidgetProps) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);

  if (variant === "board") {
    return <BoardGrid source={source} count={count} title={title} variant={variant} />;
  }

  return (
    <div className="my-3">
      {title && (
        <div className="text-sm font-medium text-base-content/80 mb-2">
          {title}
        </div>
      )}
      <InlinePreview
        source={source}
        count={count}
        onOpen={() => setExpanded(true)}
      />
      <button
        className="inline-flex items-center gap-1.5 mt-2 px-3 py-1.5 text-xs font-medium rounded-full border border-primary/30 text-primary hover:bg-primary/10 transition-all"
        onClick={() => setExpanded(true)}
      >
        <Images size={14} strokeWidth={1.5} />
        {t("lumilio.widgets.viewAll", "View all {{count}} photos", { count })}
      </button>
      {expanded && (
        <GridModal
          source={source}
          count={count}
          title={title}
          onClose={() => setExpanded(false)}
        />
      )}
    </div>
  );
}

function InlinePreview({
  source,
  count,
  onOpen,
}: Pick<WidgetProps, "source" | "count"> & { onOpen: () => void }) {
  const { t } = useI18n();
  const { assets, isLoading, isError } = useWidgetAssetsPreview(
    source,
    INLINE_PREVIEW_COUNT,
  );

  if (isError) {
    return (
      <div className="rounded-xl border border-base-300 bg-base-200/40 p-4 max-w-md text-sm text-base-content/60">
        {t(
          "lumilio.widgets.refExpired",
          "These results have expired. Ask Lumilio to run the search again.",
        )}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-4 gap-1 max-w-md rounded-xl overflow-hidden">
      {isLoading
        ? Array.from({ length: Math.min(count, INLINE_PREVIEW_COUNT) }).map(
            (_, i) => (
              <div key={i} className="aspect-square bg-base-200 animate-pulse" />
            ),
          )
        : assets.map((asset, i) => {
            const isLastTile =
              i === INLINE_PREVIEW_COUNT - 1 && count > INLINE_PREVIEW_COUNT;
            return (
              <button
                key={asset.asset_id}
                className="relative aspect-square overflow-hidden cursor-pointer"
                onClick={onOpen}
              >
                <img
                  src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
                  alt=""
                  loading="lazy"
                  className="w-full h-full object-cover hover:scale-105 transition-transform duration-200"
                />
                {isLastTile && (
                  <span className="absolute inset-0 bg-black/55 flex items-center justify-center text-white text-sm font-medium">
                    +{count - INLINE_PREVIEW_COUNT + 1}
                  </span>
                )}
              </button>
            );
          })}
    </div>
  );
}

/** Board cell: a self-scrolling grid that paginates as you scroll. */
function BoardGrid({ source, count }: WidgetProps) {
  const { t } = useI18n();
  const [carouselAssetId, setCarouselAssetId] = useState<string>();
  const query = useWidgetAssetsInfinite(source, PAGE_SIZE);

  const assets = useMemo<Asset[]>(
    () =>
      (query.data?.pages ?? []).flatMap(
        (page) => (page as ApiResult<AgentRefAssetsDTO>)?.data?.assets ?? [],
      ),
    [query.data],
  );

  const slideIndex = useMemo(
    () =>
      carouselAssetId
        ? assets.findIndex((a) => a.asset_id === carouselAssetId)
        : -1,
    [assets, carouselAssetId],
  );

  const handleScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const el = event.currentTarget;
      if (
        el.scrollHeight - el.scrollTop - el.clientHeight < 400 &&
        query.hasNextPage &&
        !query.isFetchingNextPage
      ) {
        void query.fetchNextPage();
      }
    },
    [query],
  );

  if (query.isError) {
    return (
      <div className="h-full flex items-center justify-center text-xs text-base-content/50 p-3 text-center">
        {t(
          "lumilio.widgets.pinUnavailable",
          "This widget's photos are unavailable.",
        )}
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto" onScroll={handleScroll}>
      <div className="grid grid-cols-3 gap-0.5">
        {assets.map((asset) => (
          <button
            key={asset.asset_id}
            className="aspect-square overflow-hidden cursor-pointer"
            onClick={() => setCarouselAssetId(asset.asset_id)}
          >
            <img
              src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
              alt=""
              loading="lazy"
              className="w-full h-full object-cover hover:opacity-85 transition-opacity"
            />
          </button>
        ))}
      </div>
      {query.isFetchingNextPage && (
        <div className="flex justify-center py-2">
          <span className="w-4 h-4 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
        </div>
      )}
      <span className="sr-only">{count}</span>

      {carouselAssetId &&
        assets.length > 0 &&
        createPortal(
          <FullScreenCarousel
            photos={assets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={() => setCarouselAssetId(undefined)}
            onNavigate={setCarouselAssetId}
          />,
          document.body,
        )}
    </div>
  );
}

function GridModal({
  source,
  count,
  title,
  onClose,
}: Pick<WidgetProps, "source" | "count" | "title"> & { onClose: () => void }) {
  const { t } = useI18n();
  const [carouselAssetId, setCarouselAssetId] = useState<string>();
  const query = useWidgetAssetsInfinite(source, PAGE_SIZE);

  const assets = useMemo<Asset[]>(
    () =>
      (query.data?.pages ?? []).flatMap(
        (page) => (page as ApiResult<AgentRefAssetsDTO>)?.data?.assets ?? [],
      ),
    [query.data],
  );

  const slideIndex = useMemo(
    () =>
      carouselAssetId
        ? assets.findIndex((a) => a.asset_id === carouselAssetId)
        : -1,
    [assets, carouselAssetId],
  );

  const handleScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const el = event.currentTarget;
      if (
        el.scrollHeight - el.scrollTop - el.clientHeight < 600 &&
        query.hasNextPage &&
        !query.isFetchingNextPage
      ) {
        void query.fetchNextPage();
      }
    },
    [query],
  );

  return createPortal(
    <dialog className="modal modal-open">
      <div className="modal-box w-11/12 max-w-6xl h-[88vh] p-5 flex flex-col bg-base-100 shadow-2xl">
        <div className="flex items-center justify-between mb-4 shrink-0">
          <h3 className="font-semibold text-base-content flex items-center gap-2">
            <Images size={18} strokeWidth={1.5} className="text-primary" />
            {title ?? t("lumilio.widgets.resultsTitle", "Photos from Lumilio")}
            <span className="text-sm font-normal text-base-content/50">
              {count}
            </span>
          </h3>
          <button className="btn btn-sm btn-ghost btn-circle" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto" onScroll={handleScroll}>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-1">
            {assets.map((asset) => (
              <button
                key={asset.asset_id}
                className="aspect-square overflow-hidden cursor-pointer"
                onClick={() => setCarouselAssetId(asset.asset_id)}
              >
                <img
                  src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
                  alt=""
                  loading="lazy"
                  className="w-full h-full object-cover hover:opacity-85 transition-opacity"
                />
              </button>
            ))}
          </div>
          {query.isFetchingNextPage && (
            <div className="flex justify-center py-4">
              <span className="w-5 h-5 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
            </div>
          )}
        </div>
      </div>
      <form method="dialog" className="modal-backdrop" onClick={onClose}>
        <button>{t("common.close")}</button>
      </form>

      {carouselAssetId &&
        assets.length > 0 &&
        createPortal(
          <FullScreenCarousel
            photos={assets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={() => setCarouselAssetId(undefined)}
            onNavigate={setCarouselAssetId}
          />,
          document.body,
        )}
    </dialog>,
    document.body,
  );
}

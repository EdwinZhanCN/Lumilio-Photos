import { useCallback, useState, useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { useParams } from "react-router-dom";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import SquareGallery from "@/features/assets/components/page/SquareGallery/SquareGallery";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import {
  useCurrentAssetsView,
  useCurrentAssetsSearchView,
} from "@/features/assets/hooks/useAssetsView";
import {
  useSortBy,
  useIsCarouselOpen,
  useSearchQuery,
  useUIActions,
} from "@/features/assets/selectors";
import { useI18n } from "@/lib/i18n";
import { useSettingsContext } from "@/features/settings";
import type { BrowseGroup } from "@/features/assets";
import type { AssetGalleryProps } from "./gallery.types";
import {
  findBrowseItemIndexByAssetId,
} from "@/features/assets/utils/browseItems";

export function AssetsGalleryPage() {
  const { assetId } = useParams<{ assetId: string }>();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const { t } = useI18n();
  const { state: settingsState } = useSettingsContext();

  // Selectors
  const sortBy = useSortBy();
  const searchQuery = useSearchQuery();
  const isCarouselOpen = useIsCarouselOpen();
  const { setSortBy } = useUIActions();
  const isSearchActive = searchQuery.trim().length > 0;
  const currentLayout = settingsState.ui.asset_page?.layout ?? "full";
  const compactColumns = settingsState.ui.asset_page?.columns ?? 6;
  const GalleryComponent =
    currentLayout === "compact" ? SquareGallery : JustifiedGallery;

  const {
    assets: allAssets,
    browseGroups,
    browseItems: flatBrowseItems,
    browseAssets: flatAssets,
    isLoading: isFetching,
    isLoadingMore: isFetchingNextPage,
    fetchMore: fetchNextPage,
    hasMore: hasNextPage,
    isFetched,
    error,
  } = useCurrentAssetsView({
    withGroups: true,
    sortBy,
  });
  const photoSearchView = useCurrentAssetsSearchView({
    withGroups: false,
    sortBy,
  });
  const [lastBrowseGroups, setLastBrowseGroups] = useState<BrowseGroup[] | null>(
    null,
  );

  const hasFetchedOnce = isFetched;

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const topResultsBrowseGroups = photoSearchView.topResultsBrowseGroups;
  const searchResultBrowseGroups = photoSearchView.resultBrowseGroups;
  const activeBrowseGroups = isSearchActive ? [] : browseGroups;
  const activeBrowseItems = isSearchActive
    ? photoSearchView.browseItems
    : flatBrowseItems;
  const activeBrowseAssets = isSearchActive
    ? photoSearchView.browseAssets
    : flatAssets;

  useEffect(() => {
    if (!isSearchActive && activeBrowseGroups.length > 0) {
      setLastBrowseGroups(activeBrowseGroups);
    }
  }, [activeBrowseGroups, isSearchActive]);

  const showSearchTransitionOverlay =
    isSearchActive &&
    isFetching &&
    allAssets.length === 0 &&
    lastBrowseGroups !== null &&
    lastBrowseGroups.length > 0;

  const renderSearchSections = () => {
    if (showSearchTransitionOverlay && lastBrowseGroups) {
      return (
        <div className="relative">
          <GalleryComponent
            key={`search-transition:${currentLayout}:${compactColumns}`}
            browseGroups={lastBrowseGroups}
            openCarousel={openCarousel}
            onLoadMore={() => {}}
            hasMore={false}
            isLoadingMore={false}
            columns={compactColumns}
            className="pointer-events-none opacity-45 transition-opacity duration-200"
          />
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-base-100/35 backdrop-blur-[2px]">
            <div className="rounded-full border border-base-300/80 bg-base-100/90 px-4 py-2 shadow-sm">
              <span className="inline-flex items-center gap-2 text-sm text-base-content/70">
                <span className="loading loading-spinner loading-sm"></span>
                {t("search.loading", {
                  defaultValue: "Searching...",
                })}
              </span>
            </div>
          </div>
        </div>
      );
    }

    if (isFetching && allAssets.length === 0) {
      return <PhotosLoadingSkeleton />;
    }

    return (
      <div className="space-y-6">
        {photoSearchView.topResultsMeta.degraded && (
          <div className="px-4">
            <div className="alert alert-info border border-info/20 bg-info/10 text-info-content">
              <span>
                {t("search.degraded", {
                  defaultValue:
                    "Top results are temporarily unavailable. Showing regular results instead.",
                })}
              </span>
            </div>
          </div>
        )}

        {error && (
          <div className="px-4">
            <div className="alert alert-warning">
              <span>
                {t("search.error", {
                  defaultValue:
                    "Search is temporarily unavailable. Try again in a moment.",
                })}
              </span>
            </div>
          </div>
        )}

        {photoSearchView.topResults.length > 0 && (
          <GalleryComponent
            key={`search-top:${currentLayout}`}
            browseGroups={topResultsBrowseGroups}
            openCarousel={openCarousel}
            onLoadMore={() => {}}
            hasMore={false}
            isLoadingMore={false}
            columns={compactColumns}
          />
        )}

        {(!error || photoSearchView.resultAssets.length > 0) && (
          <GalleryComponent
            key={`search-results:${currentLayout}`}
            browseGroups={searchResultBrowseGroups}
            openCarousel={openCarousel}
            onLoadMore={handleLoadMore}
            hasMore={hasNextPage}
            isLoadingMore={isFetchingNextPage}
            isLoading={isFetching && photoSearchView.resultAssets.length === 0}
            columns={compactColumns}
          />
        )}
      </div>
    );
  };

  // Carousel positioning logic
  const [slideIndex, setSlideIndex] = useState<number>(-1);
  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  useEffect(() => {
    if (!isCarouselOpen) return;
    if (assetId && hasFetchedOnce) {
      const index = findBrowseItemIndexByAssetId(activeBrowseItems, assetId);
      if (index >= 0) {
        setSlideIndex(index);
        setIsLocatingAsset(false);
        return;
      }
      setIsLocatingAsset(true);
      if (hasNextPage && !isFetching && !isFetchingNextPage) {
        fetchNextPage();
        return;
      }
      if (!hasNextPage && !isFetching && !isFetchingNextPage) {
        const timer = setTimeout(() => closeCarousel(), 500);
        return () => clearTimeout(timer);
      }
    }
  }, [
    assetId,
    activeBrowseItems,
    isCarouselOpen,
    hasFetchedOnce,
    hasNextPage,
    isFetching,
    isFetchingNextPage,
    fetchNextPage,
    closeCarousel,
  ]);

  useEffect(() => {
    if (assetId && activeBrowseAssets.length > 0) {
      const index = findBrowseItemIndexByAssetId(activeBrowseItems, assetId);
      if (index >= 0) {
        setSlideIndex(index);
        setIsLocatingAsset(false);
      }
    }
  }, [
    assetId,
    activeBrowseAssets.length,
    activeBrowseItems,
  ]);

  // Render an inline error rather than throwing, so a transient API failure
  // doesn't trigger the full-screen ErrorBoundary and lock the user out.
  if (error && !isSearchActive) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 p-16 text-center">
        <AlertTriangle className="size-8 text-warning" />
        <p className="text-base-content/70">
          {t("assets.all.load_error", {
            defaultValue: "Failed to load photos. Check your connection and try again.",
          })}
        </p>
        <button
          className="btn btn-sm btn-outline"
          onClick={() => fetchNextPage()}
        >
          {t("common.retry", { defaultValue: "Retry" })}
        </button>
      </div>
    );
  }

  const showEndOfResults =
    !hasNextPage &&
    (isSearchActive
      ? photoSearchView.resultAssets.length > 0
      : allAssets.length > 0);

  const browseGalleryProps: AssetGalleryProps = {
    browseGroups: activeBrowseGroups,
    openCarousel,
    onLoadMore: handleLoadMore,
    hasMore: hasNextPage,
    isLoadingMore: isFetchingNextPage,
    columns: compactColumns,
  };

  return (
    <div>
      <AssetsPageHeader
        sortBy={sortBy}
        onSortByChange={setSortBy}
        onFiltersChange={() => {}}
        browseItems={activeBrowseItems}
      />

      {isSearchActive ? (
        renderSearchSections()
      ) : isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <GalleryComponent
          key={`browse:${currentLayout}`}
          {...browseGalleryProps}
        />
      )}

      {showEndOfResults && (
        <div className="text-center p-4 text-gray-500">
          {t("assets.all.end_of_results")}
        </div>
      )}

      {isCarouselOpen &&
        (activeBrowseAssets.length > 0 ? (
          <>
            <FullScreenCarousel
              photos={activeBrowseAssets}
              initialSlide={slideIndex >= 0 ? slideIndex : 0}
              slideIndex={slideIndex >= 0 ? slideIndex : undefined}
              onClose={closeCarousel}
              onNavigate={openCarousel}
            />
            {isLocatingAsset && (
              <div className="fixed inset-0 bg-black/70 z-[60] flex items-center justify-center">
                <div className="text-white text-center bg-black/50 backdrop-blur-sm rounded-2xl p-8 max-w-md">
                  <div className="loading loading-spinner loading-lg mb-4"></div>
                  <p className="text-lg font-medium mb-2">
                    {t("assets.all.locating_asset")}
                  </p>
                  {hasNextPage && !isFetching && !isFetchingNextPage ? (
                    <p className="text-sm text-gray-300">
                      {t("assets.all.loading_more_data")}
                    </p>
                  ) : (
                    <p className="text-sm text-gray-300">
                      {t("assets.all.asset_not_available")}
                    </p>
                  )}
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center">
            <div className="text-white text-center">
              <div className="loading loading-spinner loading-lg mb-4"></div>
              <p>{t("assets.all.loading_assets")}</p>
            </div>
          </div>
        ))}
    </div>
  );
}

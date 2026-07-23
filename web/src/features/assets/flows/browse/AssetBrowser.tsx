import { useCallback, useState, useEffect, useMemo, type ReactNode } from "react";
import { AlertTriangle, ArrowLeft, Pin } from "lucide-react";
import { Link, useLocation, useParams } from "react-router-dom";
import { useDockStore } from "@/lib/assistant";
import { usePreference } from "@/lib/preferences/preferences";
import AssetsPageHeader from "./header/AssetsPageHeader";
import type { AssetsBulkActionId, AssetsBulkActionInput } from "@/lib/assets/bulkActions";
import AssetViewer from "../viewer/AssetViewer";
import JustifiedGallery from "./gallery/JustifiedGallery/JustifiedGallery";
import SquareGallery from "./gallery/SquareGallery/SquareGallery";
import PhotosLoadingSkeleton from "./gallery/LoadingSkeleton";
import { SearchFAB } from "./SearchFAB";
import { useAssetBrowserNavigation } from "./selection/AssetBrowserScope";
import { useAssetBrowser } from "./useAssetBrowser";
import { useBrowseSelectionContext } from "./useBrowseSelectionContext";
import { usePinAssetsView } from "../../api/usePinAssetsView";
import { useI18n } from "@/lib/i18n";
import type { BrowseGroup } from "../../types";
import type { AssetGalleryProps } from "./gallery/gallery.types";
import { findBrowseItemIndexByAssetId } from "../../model/browseItems";
import { countActiveAssetUserFilters, type AssetBrowseConstraint } from "../../model/filter";
import { useAssetBrowseRouteState } from "./useAssetBrowseRouteState";

type PinNavigationOrigin = {
  from?: string;
  fromLabel?: string;
};

export type AssetBrowserProps = {
  title?: string;
  icon?: ReactNode;
  constraint?: AssetBrowseConstraint;
  bulkActions?: AssetsBulkActionInput;
  hiddenBulkActions?: readonly AssetsBulkActionId[];
  viewKey?: string;
  /** When set, the gallery switches to pin-driven mode and hydrates assets
   * from the saved agent result (pin) instead of the normal browse view. */
  pinId?: string;
  /** Custom banner rendered between the header and the gallery (album info,
   * person cover, trip map, etc.). Lets scoped collections reuse this page
   * instead of hand-rolling header + carousel + search. */
  hero?: ReactNode;
  /** Whether scoped search (FAB + search view) is available. Off for views
   * whose data source can't be searched. */
  searchEnabled?: boolean;
  /** Main-library-only bridge from the retired persisted browse store to URL state. */
  migrateLegacyState?: boolean;
};

export function AssetBrowser({
  title,
  icon,
  constraint,
  bulkActions,
  hiddenBulkActions,
  viewKey,
  pinId,
  hero,
  searchEnabled = true,
  migrateLegacyState = false,
}: AssetBrowserProps = {}) {
  const { assetId } = useParams<{ assetId: string }>();
  const location = useLocation();
  const pinOrigin = (location.state ?? null) as PinNavigationOrigin | null;
  const { openViewer, replaceViewerAsset, closeViewer } = useAssetBrowserNavigation();
  const { t } = useI18n();
  const [assetPage] = usePreference("assetPage");
  const isPinMode = Boolean(pinId);

  // Hide the search FAB while the agent dock is expanded (the panel sits over it).
  const dockExpanded = useDockStore((s) => s.collapsedOverride) === false;

  const {
    query: searchQuery,
    sort: sortBy,
    filter: userFilter,
    setQuery,
    setSort,
    applyFilter,
  } = useAssetBrowseRouteState({ migrateLegacyState });
  const isCarouselOpen = Boolean(assetId);
  const isSearchActive = searchEnabled && searchQuery.trim().length > 0;
  const hasActiveFilters = countActiveAssetUserFilters(userFilter) > 0;
  const isTrashView = constraint?.is_deleted === true;
  const emptyState = useMemo(() => {
    if (isSearchActive) {
      return {
        title: t("assets.all.emptySearchTitle"),
        description: t("assets.all.emptySearchDescription"),
      };
    }
    if (isTrashView) {
      return {
        title: t("assets.trash.emptyTitle"),
        description: t("assets.trash.emptyDescription"),
      };
    }
    if (hasActiveFilters) {
      return {
        title: t("assets.all.emptyFilteredTitle"),
        description: t("assets.all.emptyFilteredDescription"),
      };
    }
    return {
      title: t("assets.all.emptyTitle"),
      description: t("assets.all.emptyDescription"),
    };
  }, [hasActiveFilters, isSearchActive, isTrashView, t]);
  const currentLayout = assetPage.layout;
  const compactColumns = assetPage.columns;
  const GalleryComponent = currentLayout === "compact" ? SquareGallery : JustifiedGallery;

  const pinView = usePinAssetsView(pinId, {
    sortBy,
    constraint,
    userFilter,
    viewKey,
    searchQuery,
    searchEnabled,
  });
  const standardView = useAssetBrowser({
    withGroups: true,
    sortBy,
    constraint,
    userFilter,
    searchQuery: searchEnabled ? searchQuery : "",
    viewKey,
    disabled: isPinMode,
  });

  // Pin mode hydrates from a saved agent result; otherwise the standard browse
  // view scoped by the page constraint.
  const activeView = isPinMode ? pinView : standardView;
  const activeSearchView = activeView;
  const {
    assets: allAssets,
    browseGroups,
    browseItems: flatBrowseItems,
    browseAssets: flatAssets,
    isLoading: isFetching,
    isLoadingMore: isFetchingNextPage,
    fetchMore: fetchNextPage,
    refetch: refetchView,
    hasMore: hasNextPage,
    isFetched,
    error,
  } = activeView;
  const [lastBrowseGroups, setLastBrowseGroups] = useState<BrowseGroup[] | null>(null);

  const hasFetchedOnce = isFetched;

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const topResultsBrowseGroups = activeSearchView.topResultsBrowseGroups;
  const searchResultBrowseGroups = activeSearchView.resultBrowseGroups;
  const activeBrowseGroups = isSearchActive ? [] : browseGroups;
  const activeBrowseItems = flatBrowseItems;
  const activeBrowseAssets = flatAssets;

  // Gallery selection becomes agent context. Selection state stores browse item
  // ids; resolve them to asset UUIDs before the chat request snapshots context.
  useBrowseSelectionContext(activeBrowseItems);

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
            openCarousel={openViewer}
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
        {activeSearchView.topResultsMeta.degraded && (
          <div className="px-4">
            <div className="alert alert-info border border-info/20 bg-info/10 text-info-content">
              <span>
                {t("search.semanticUnavailable", {
                  defaultValue:
                    "Semantic search is temporarily unavailable; results may be less complete.",
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
                  defaultValue: "Search is temporarily unavailable. Try again in a moment.",
                })}
              </span>
            </div>
          </div>
        )}

        {activeSearchView.topResults.length > 0 && (
          <GalleryComponent
            key={`search-top:${currentLayout}`}
            browseGroups={topResultsBrowseGroups}
            openCarousel={openViewer}
            onLoadMore={() => {}}
            hasMore={false}
            isLoadingMore={false}
            columns={compactColumns}
          />
        )}

        {(!error || activeSearchView.resultAssets.length > 0) && (
          <GalleryComponent
            key={`search-results:${currentLayout}`}
            browseGroups={searchResultBrowseGroups}
            openCarousel={openViewer}
            onLoadMore={handleLoadMore}
            hasMore={hasNextPage}
            isLoadingMore={isFetchingNextPage}
            isLoading={isFetching && activeSearchView.resultAssets.length === 0}
            columns={compactColumns}
            emptyStateTitle={emptyState.title}
            emptyStateDescription={emptyState.description}
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
        void fetchNextPage();
        return;
      }
      if (!hasNextPage && !isFetching && !isFetchingNextPage) {
        const timer = setTimeout(() => closeViewer(), 500);
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
    closeViewer,
  ]);

  useEffect(() => {
    if (assetId && activeBrowseAssets.length > 0) {
      const index = findBrowseItemIndexByAssetId(activeBrowseItems, assetId);
      if (index >= 0) {
        setSlideIndex(index);
        setIsLocatingAsset(false);
      }
    }
  }, [assetId, activeBrowseAssets.length, activeBrowseItems]);

  const pinHeader = useMemo(() => {
    if (!isPinMode) {
      return { title: undefined, subtitle: undefined, icon: undefined };
    }

    const pin = pinView.pin;
    const resolvedTitle =
      pin?.title || t("assets.pin.defaultTitle", { defaultValue: "Agent result" });
    const metaParts: string[] = [];

    if (pin?.mode) {
      metaParts.push(pin.mode === "live" ? t("assets.pin.modeLive") : t("assets.pin.modeFrozen"));
    }
    if ((pin?.count ?? 0) > 0) {
      metaParts.push(t("assets.pin.count", { count: pin?.count ?? 0 }));
    }

    const metaLine = metaParts.join(" · ");
    const summary = pin?.summary?.trim();
    const resolvedSubtitle = summary
      ? metaLine
        ? `${metaLine} — ${summary}`
        : summary
      : metaLine || undefined;

    return {
      title: resolvedTitle,
      subtitle: resolvedSubtitle,
      icon: <Pin className="h-6 w-6 text-primary" />,
    };
  }, [isPinMode, pinView.pin, t]);

  // Pin expired/deleted: the metadata lookup failed (typically 404). Retry
  // can't recover a missing pin, so offer a way back to Lumilio instead.
  if (isPinMode && pinView.isExpired) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 p-16 text-center">
        <AlertTriangle className="size-8 text-warning" />
        <p className="max-w-md text-base-content/70">{t("assets.pin.expired")}</p>
        <Link to={pinOrigin?.from ?? "/lumilio"} className="btn btn-sm btn-outline gap-1.5">
          <ArrowLeft className="size-4" />
          {pinOrigin?.fromLabel ?? t("assets.pin.backToLumilio")}
        </Link>
      </div>
    );
  }

  // Render an inline error rather than throwing, so a transient API failure
  // doesn't trigger the full-screen ErrorBoundary and lock the user out.
  if (error && !isSearchActive) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 p-16 text-center">
        <AlertTriangle className="size-8 text-warning" />
        <p className="text-base-content/70">
          {t("assets.all.load_error", {
            defaultValue: "Failed to load photos. Check your connection and try again.",
          })}
        </p>
        <button
          className="btn btn-sm btn-outline"
          onClick={() => {
            if (isPinMode) {
              void refetchView();
            } else {
              void fetchNextPage();
            }
          }}
        >
          {t("common.retry", { defaultValue: "Retry" })}
        </button>
      </div>
    );
  }

  const showEndOfResults =
    !hasNextPage &&
    (isSearchActive ? activeSearchView.resultAssets.length > 0 : allAssets.length > 0);

  const browseGalleryProps: AssetGalleryProps = {
    browseGroups: activeBrowseGroups,
    openCarousel: openViewer,
    onLoadMore: handleLoadMore,
    hasMore: hasNextPage,
    isLoadingMore: isFetchingNextPage,
    columns: compactColumns,
    emptyStateTitle: emptyState.title,
    emptyStateDescription: emptyState.description,
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <AssetsPageHeader
        sortBy={sortBy}
        onSortByChange={setSort}
        filter={userFilter}
        constraint={constraint}
        onFiltersChange={applyFilter}
        title={isPinMode ? pinHeader.title : title}
        subtitle={isPinMode ? pinHeader.subtitle : undefined}
        icon={isPinMode ? pinHeader.icon : icon}
        browseItems={activeBrowseItems}
        bulkActions={bulkActions}
        hiddenBulkActions={hiddenBulkActions}
        capabilities={{ showScan: !isPinMode }}
        scopeControlHidden={constraint?.repository_id !== undefined}
      />

      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
        {hero}

        {isSearchActive ? (
          renderSearchSections()
        ) : isFetching && allAssets.length === 0 ? (
          <PhotosLoadingSkeleton />
        ) : (
          <GalleryComponent key={`browse:${currentLayout}`} {...browseGalleryProps} />
        )}

        {showEndOfResults && (
          <div className="text-center p-4 text-gray-500">{t("assets.all.end_of_results")}</div>
        )}
      </div>

      {isCarouselOpen &&
        (activeBrowseAssets.length > 0 ? (
          <>
            <AssetViewer
              photos={activeBrowseAssets}
              initialSlide={slideIndex >= 0 ? slideIndex : 0}
              slideIndex={slideIndex >= 0 ? slideIndex : undefined}
              onClose={closeViewer}
              onNavigate={replaceViewerAsset}
            />
            {isLocatingAsset && (
              <div className="fixed inset-0 bg-black/70 z-overlay flex items-center justify-center">
                <div className="text-white text-center bg-black/50 backdrop-blur-sm rounded-2xl p-8 max-w-md">
                  <div className="loading loading-spinner loading-lg mb-4"></div>
                  <p className="text-lg font-medium mb-2">{t("assets.all.locating_asset")}</p>
                  {hasNextPage && !isFetching && !isFetchingNextPage ? (
                    <p className="text-sm text-gray-300">{t("assets.all.loading_more_data")}</p>
                  ) : (
                    <p className="text-sm text-gray-300">{t("assets.all.asset_not_available")}</p>
                  )}
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="fixed inset-0 bg-black/90 z-lightbox flex items-center justify-center">
            <div className="text-white text-center">
              <div className="loading loading-spinner loading-lg mb-4"></div>
              <p>{t("assets.all.loading_assets")}</p>
            </div>
          </div>
        ))}
      {!isCarouselOpen && !dockExpanded && searchEnabled && (
        <SearchFAB query={searchQuery} onQueryChange={setQuery} />
      )}
    </div>
  );
}

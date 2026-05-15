import { useParams, useLocation } from "react-router-dom";
import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { MapPin } from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import {
  useSortBy,
  useIsCarouselOpen,
  useUIActions,
} from "@/features/assets/selectors";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import { JustifiedGallery } from "@/features/assets";
import { findBrowseItemIndexByAssetId } from "@/features/assets/utils/browseItems";
import { useI18n } from "@/lib/i18n.tsx";
import type { AssetViewDefinition } from "@/features/assets/types/assets.type";
import type { CityTripGroup } from "../hooks/useCityTrips";
import { useCityTrips } from "../hooks/useCityTrips";
import { useWorkingRepository } from "@/features/settings";

function formatDateRange(start: Date, end: Date, locale?: string): string {
  const resolvedLocale = locale || "en";
  const sameDay = start.toDateString() === end.toDateString();

  if (sameDay) {
    return start.toLocaleDateString(resolvedLocale, {
      month: "long",
      day: "numeric",
      year: "numeric",
    });
  }

  const sameYear = start.getFullYear() === end.getFullYear();
  if (sameYear) {
    return `${start.toLocaleDateString(resolvedLocale, { month: "long", day: "numeric" })} – ${end.toLocaleDateString(resolvedLocale, { month: "long", day: "numeric", year: "numeric" })}`;
  }

  return `${start.toLocaleDateString(resolvedLocale, { month: "long", day: "numeric", year: "numeric" })} – ${end.toLocaleDateString(resolvedLocale, { month: "long", day: "numeric", year: "numeric" })}`;
}

const TripAssetsContent = ({ trip }: { trip: CityTripGroup }) => {
  const { t, i18n } = useI18n();
  const { assetId } = useParams<{ assetId: string }>();
  const sortBy = useSortBy();
  const isCarouselOpen = useIsCarouselOpen();
  const { setSortBy } = useUIActions();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const [isScrolled, setIsScrolled] = useState(false);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const locale = i18n.resolvedLanguage || i18n.language;

  const viewDefinition: AssetViewDefinition = useMemo(
    () => ({
      filter: {
        location: {
          north: trip.bbox.north,
          south: trip.bbox.south,
          east: trip.bbox.east,
          west: trip.bbox.west,
        },
        date: {
          from: trip.startTime.toISOString(),
          to: trip.endTime.toISOString(),
        },
      },
      sortBy,
      pageSize: 50,
    }),
    [trip.bbox, trip.startTime, trip.endTime, sortBy],
  );

  const {
    assets,
    browseGroups,
    browseItems,
    browseAssets: flatAssets,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error,
  } = useAssetsView(viewDefinition, { withGroups: true });

  const slideIndex = useMemo(() => {
    if (assetId && flatAssets.length > 0) {
      return findBrowseItemIndexByAssetId(browseItems, assetId);
    }
    return -1;
  }, [assetId, browseItems, flatAssets.length]);

  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  useEffect(() => {
    if (isCarouselOpen && assetId && flatAssets.length > 0) {
      const index = findBrowseItemIndexByAssetId(browseItems, assetId);
      if (index < 0) {
        if (hasMore && !isLoading && !isLoadingMore) {
          setIsLocatingAsset(true);
          fetchMore();
        }
      } else {
        setIsLocatingAsset(false);
      }
    }
  }, [
    assetId,
    flatAssets,
    browseItems,
    isCarouselOpen,
    hasMore,
    isLoading,
    isLoadingMore,
    fetchMore,
  ]);

  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore) {
      fetchMore();
    }
  }, [hasMore, isLoadingMore, fetchMore]);

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    const top = e.currentTarget.scrollTop;
    setIsScrolled((prev) => (prev ? top > 20 : top > 60));
  }, []);

  if (error) {
    return (
      <div className="p-8 text-error">
        {t("collections.places.loadError", { error: String(error) })}
      </div>
    );
  }

  const isInitialLoading = isLoading && assets.length === 0;

  return (
    <div className="flex h-full flex-col relative">
      <div className="sticky top-0 z-30 border-b border-base-200/30 bg-base-100/80 backdrop-blur-md">
        <AssetsPageHeader
          sortBy={sortBy}
          onSortByChange={setSortBy}
          title={trip.displayTitle}
          icon={<MapPin className="w-6 h-6 text-primary" />}
          browseItems={browseItems}
        />

        <div
          className={`overflow-hidden px-4 transition-all duration-500 ease-in-out ${isScrolled ? "py-1.5" : "py-4"}`}
        >
          <div
            className={`transition-all duration-500 ease-in-out ${isScrolled ? "max-h-0 opacity-0 -translate-y-2" : "max-h-20 opacity-100 translate-y-0"}`}
          >
            <div className="flex items-baseline gap-4">
              <h1 className="truncate text-4xl font-black tracking-tight text-primary">
                {trip.displayTitle}
              </h1>
            </div>
          </div>

          <div
            className={`flex items-center gap-6 transition-all duration-500 ease-in-out ${isScrolled ? "mt-0 text-[10px] opacity-60" : "mt-6 text-xs opacity-40"}`}
          >
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              <span>
                {t("collections.itemsCount", { count: trip.photoCount })}
              </span>
            </div>
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              <span>
                {formatDateRange(trip.startTime, trip.endTime, locale)}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div
        ref={scrollContainerRef}
        className="flex-1 overflow-y-auto"
        onScroll={handleScroll}
      >
        <div>
          {isInitialLoading ? (
            <PhotosLoadingSkeleton />
          ) : (
            <JustifiedGallery
              browseGroups={browseGroups}
              openCarousel={openCarousel}
              onLoadMore={handleLoadMore}
              hasMore={hasMore}
              isLoadingMore={isLoadingMore}
            />
          )}
        </div>
      </div>

      {isCarouselOpen && flatAssets.length > 0 && (
        <>
          <FullScreenCarousel
            photos={flatAssets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={closeCarousel}
            onNavigate={openCarousel}
          />
          {isLocatingAsset && (
            <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/70">
              <div className="max-w-md rounded-2xl bg-black/50 p-8 text-center text-white backdrop-blur-sm">
                <div className="loading loading-spinner loading-lg mb-4"></div>
                <p className="mb-2 text-lg font-medium">
                  {t("assets.all.locating_asset")}
                </p>
                <p className="text-sm text-gray-300">
                  {t("assets.all.loading_more_data")}
                </p>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
};

const TripDetails = () => {
  const { tripId } = useParams<{
    tripId: string;
    assetId: string;
  }>();
  const location = useLocation();
  const { scopedRepositoryId } = useWorkingRepository();
  const { trips } = useCityTrips({ repositoryId: scopedRepositoryId });

  // Try to get trip from route state first, fall back to lookup by id
  const routeTrip = (location.state as { trip?: CityTripGroup } | null)?.trip;
  const trip = useMemo(() => {
    if (routeTrip) return routeTrip;
    return trips.find((t) => t.id === tripId);
  }, [routeTrip, trips, tripId]);

  if (!trip) {
    if (trips.length === 0) {
      return (
        <div className="flex h-full items-center justify-center">
          <div className="loading loading-spinner loading-lg"></div>
        </div>
      );
    }
    return (
      <div className="flex h-full items-center justify-center text-base-content/60">
        <p>Trip not found.</p>
      </div>
    );
  }

  return (
    <WorkerProvider preload={["exif", "export"]}>
      <AssetsProvider
        key={`trip:${trip.id}`}
        scopeId={`trip:${trip.id}`}
        persist={false}
        basePath={`/collections/places/${trip.id}`}
      >
        <TripAssetsContent trip={trip} />
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default TripDetails;

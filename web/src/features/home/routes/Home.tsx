import { lazy, Suspense } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import PageHeader from "@/components/ui/PageHeader";
import { BrowseScopeSelect, useBrowseScope } from "@/features/repositories";
import GalleryGrid from "../components/GalleryGrid";
import StatsCards from "../components/StatsCards";
import { useLocationClusters, useMapPhotoAssets } from "@/features/assets/map";
import { useI18n } from "@/lib/i18n.tsx";
import { useFeaturedPhotos } from "../api/useFeaturedPhotos";
import { AlertTriangleIcon, CameraIcon, HomeIcon, SparklesIcon } from "lucide-react";
import { useVisibleOnce } from "@/lib/utils/useVisibleOnce";

const SpacetimeMapCard = lazy(() => import("../components/SpacetimeMapCard"));

function Home() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { scopedRepositoryId } = useBrowseScope();
  const displayMode = searchParams.get("tab") === "stats" ? "stats" : "gallery";
  const [mapRef, mapVisible] = useVisibleOnce("400px 0px");

  const setDisplayMode = (nextMode: "gallery" | "stats") => {
    const params = new URLSearchParams(searchParams);

    if (nextMode === "gallery") {
      params.delete("tab");
    } else {
      params.set("tab", nextMode);
    }

    setSearchParams(params, { replace: true });
  };

  const {
    assets: featuredAssets,
    isError,
    error,
  } = useFeaturedPhotos({
    count: 8,
    candidateLimit: 240,
    days: 3650,
    repositoryId: scopedRepositoryId,
  });
  const {
    points: mapPoints,
    loadedPhotos: mapLoadedPhotos,
    totalPhotos: mapTotalPhotos,
    isLoading: isMapLoading,
    isFetchingNextPage: isMapFetchingNextPage,
    hasNextPage: mapHasNextPage,
  } = useMapPhotoAssets({
    repositoryId: scopedRepositoryId,
    enabled: mapVisible,
    pageSize: 250,
  });
  const { loadedClusters, totalClusters } = useLocationClusters({
    repositoryId: scopedRepositoryId,
  });

  const mapSubtitle =
    isMapLoading && mapLoadedPhotos === 0
      ? t("home.map.loading")
      : mapPoints.length > 0
        ? (() => {
            let base = t("home.map.loadedStatus", {
              pointsCount: mapPoints.length,
              loadedCount: mapLoadedPhotos,
            });
            if (mapTotalPhotos) {
              base += t("home.map.loadedStatusTotal", {
                totalCount: mapTotalPhotos,
              });
            }
            if (isMapFetchingNextPage || mapHasNextPage) {
              base += t("home.map.loadedStatusMore");
            }
            return base;
          })()
        : t("home.map.empty");

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title={t("routes.home")} icon={<HomeIcon className="w-6 h-6 text-primary" />}>
        <BrowseScopeSelect />
        <div role="tablist" aria-label={t("routes.home")} className="tabs tabs-box">
          <button
            type="button"
            role="tab"
            aria-selected={displayMode === "gallery"}
            className={`tab gap-2 ${displayMode === "gallery" ? "tab-active" : ""}`}
            onClick={() => setDisplayMode("gallery")}
          >
            <SparklesIcon className="size-4" />
            {t("home.tabs.gallery")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={displayMode === "stats"}
            className={`tab gap-2 ${displayMode === "stats" ? "tab-active" : ""}`}
            onClick={() => setDisplayMode("stats")}
          >
            <CameraIcon className="size-4" />
            {t("home.tabs.stats")}
          </button>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
        {displayMode === "gallery" && (
          <div className="space-y-4">
            {isError && (
              <div className="alert alert-warning">
                <AlertTriangleIcon className="size-5" />
                <span>
                  {t("home.errors.featuredLoadFailed", {
                    message: error instanceof Error ? error.message : t("home.errors.unknown"),
                  })}
                </span>
              </div>
            )}

            <GalleryGrid assets={featuredAssets} placeholderCount={8} />
          </div>
        )}

        {displayMode === "stats" && (
          <div className="mx-4 mb-8 space-y-8 animate-fadeIn">
            <StatsCards repositoryId={scopedRepositoryId} />
          </div>
        )}

        <div ref={mapRef} className="mx-4 mb-8 min-h-72">
          {mapVisible && (
            <Suspense
              fallback={<div className="skeleton aspect-[16/9] w-full rounded-box bg-base-300" />}
            >
              <SpacetimeMapCard
                points={mapPoints}
                subtitle={mapSubtitle}
                headerRight={
                  loadedClusters > 0 ? (
                    <span className="badge badge-outline">
                      {t("home.map.placesCount", {
                        count: totalClusters ?? loadedClusters,
                      })}
                    </span>
                  ) : undefined
                }
                onPointClick={(assetId) => {
                  void navigate(`/assets/${assetId}`);
                }}
              />
            </Suspense>
          )}
        </div>
      </div>
    </div>
  );
}

export default Home;

import { useNavigate, useSearchParams } from "react-router-dom";
import {
  SparklesIcon,
  CameraIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";
import PageHeader from "@/components/PageHeader";
import GalleryGrid from "../components/GalleryGrid";
import StatsCards from "../components/StatsCards";
import SpacetimeMapCard from "../components/SpacetimeMapCard";
import InfoCard from "../components/InfoCard";
import { useI18n } from "@/lib/i18n.tsx";
import { useFeaturedPhotos } from "../hooks/useFeaturedPhotos";
import { useMapPhotoAssets } from "../hooks/useMapPhotoAssets";
import { useWorkingRepository } from "@/features/settings";

function Home() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { scopedRepositoryId } = useWorkingRepository();
  const displayMode = searchParams.get("tab") === "stats" ? "stats" : "gallery";

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
  } = useMapPhotoAssets({ repositoryId: scopedRepositoryId });

  const mapSubtitle =
    isMapLoading && mapLoadedPhotos === 0
      ? "正在加载地图数据..."
      : mapPoints.length > 0
        ? `定位照片 ${mapPoints.length} 张 / 已加载 ${mapLoadedPhotos}${mapTotalPhotos ? ` / 总计 ${mapTotalPhotos}` : ""}${isMapFetchingNextPage || mapHasNextPage ? "（继续加载中）" : ""}`
        : "暂无带地理位置的照片";

  return (
    <div className="flex flex-col h-full min-h-0">
      <PageHeader
        title={t("routes.home")}
        icon={<SparklesIcon className="w-6 h-6 text-primary" />}
      >
        <div
          role="tablist"
          aria-label={t("routes.home")}
          className="tabs tabs-box"
        >
          <button
            type="button"
            role="tab"
            aria-selected={displayMode === "gallery"}
            className={`tab gap-2 ${
              displayMode === "gallery" ? "tab-active" : ""
            }`}
            onClick={() => setDisplayMode("gallery")}
          >
            <SparklesIcon className="size-4" />
            {t("home.tabs.gallery", { defaultValue: "Gallery" })}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={displayMode === "stats"}
            className={`tab gap-2 ${
              displayMode === "stats" ? "tab-active" : ""
            }`}
            onClick={() => setDisplayMode("stats")}
          >
            <CameraIcon className="size-4" />
            {t("home.tabs.stats", { defaultValue: "Stats" })}
          </button>
        </div>
      </PageHeader>

      <div>
        {displayMode === "gallery" && (
          <div className="space-y-4">
            {isError && (
              <div className="alert alert-warning">
                <ExclamationTriangleIcon className="size-5" />
                <span>
                  featured 接口加载失败：
                  {error instanceof Error ? error.message : "Unknown error"}
                </span>
              </div>
            )}

            <GalleryGrid
              assets={featuredAssets}
              placeholderCount={8}
              onItemClick={(asset) => {
                if (!asset?.asset_id) return;
                navigate(`/assets/photos/${asset.asset_id}?groupBy=date`);
              }}
            />
          </div>
        )}

        {displayMode === "stats" && (
          <div className="mx-4 mb-8 space-y-8 animate-fadeIn">
            <StatsCards repositoryId={scopedRepositoryId} />
            <InfoCard />
          </div>
        )}

        <SpacetimeMapCard
          points={mapPoints}
          subtitle={mapSubtitle}
          onPointClick={(assetId) => {
            navigate(`/assets/photos/${assetId}?groupBy=date`);
          }}
          className="mx-4 mb-8"
        />
      </div>
    </div>
  );
}

export default Home;

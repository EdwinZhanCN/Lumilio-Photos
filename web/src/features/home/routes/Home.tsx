import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  SparklesIcon,
  CameraIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";
import GalleryGrid from "../components/GalleryGrid";
import AICategoryCarousel from "../components/AICategoryCarousel";
import FiltersCarousel from "../components/FiltersCarousel";
import StatsCards from "../components/StatsCards";
import SpacetimeMapCard from "../components/SpacetimeMapCard";
import InfoCard from "../components/InfoCard";
import { useI18n } from "@/lib/i18n.tsx";
import { isPhotoMetadata } from "@/lib/http-commons";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useFeaturedPhotos } from "../hooks/useFeaturedPhotos";

const EMPTY_EXIF = {
  camera: "-",
  lens: "-",
  aperture: "-",
  shutter: "-",
  focalLength: "-",
  iso: "-",
};

function Home() {
  const [displayMode, setDisplayMode] = useState("gallery");
  const { t } = useI18n();
  const navigate = useNavigate();

  const {
    assets: featuredAssets,
    candidateCount,
    seed,
    isLoading,
    isError,
    error,
  } = useFeaturedPhotos({
    count: 8,
    candidateLimit: 240,
    days: 3650,
  });

  const galleryItems = featuredAssets.length > 0 ? featuredAssets.length : 8;

  return (
    <div className="flex flex-col gap-8 p-6 relative">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">{t("routes.home")}</h1>
        <div className="tabs tabs-boxed bg-base-100/20 w-max backdrop-blur-lg rounded-box p-1 shadow-lg">
          <a
            className={`tab tab-lg rounded-box p-1 m-1 ${displayMode === "gallery" ? "tab-active bg-primary/20 text-primary" : ""}`}
            onClick={() => setDisplayMode("gallery")}
          >
            <SparklesIcon className="size-5 mr-2" />
            {t("home.tabs.gallery", { defaultValue: "Gallery" })}
          </a>
          <a
            className={`tab tab-lg rounded-box p-1 m-1 ${displayMode === "stats" ? "tab-active bg-primary/20 text-primary" : ""}`}
            onClick={() => setDisplayMode("stats")}
          >
            <CameraIcon className="size-5 mr-2" />
            {t("home.tabs.stats", { defaultValue: "Stats" })}
          </a>
        </div>
      </div>

      {displayMode === "gallery" && (
        <>
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
            items={galleryItems}
            titlePrefix={featuredAssets.length > 0 ? "精选照片" : "示例照片"}
            getExif={(index) => {
              const asset = featuredAssets[index];
              if (!asset || !isPhotoMetadata(asset.type, asset.specific_metadata)) {
                return EMPTY_EXIF;
              }

              const metadata = asset.specific_metadata;
              return {
                camera: metadata.camera_model || EMPTY_EXIF.camera,
                lens: metadata.lens_model || EMPTY_EXIF.lens,
                aperture:
                  typeof metadata.f_number === "number"
                    ? metadata.f_number.toFixed(1)
                    : EMPTY_EXIF.aperture,
                shutter: metadata.exposure_time || EMPTY_EXIF.shutter,
                focalLength:
                  typeof metadata.focal_length === "number"
                    ? `${Math.round(metadata.focal_length)}mm`
                    : EMPTY_EXIF.focalLength,
                iso:
                  typeof metadata.iso_speed === "number"
                    ? metadata.iso_speed
                    : EMPTY_EXIF.iso,
              };
            }}
            renderItem={
              featuredAssets.length > 0
                ? (index) => {
                    const asset = featuredAssets[index];
                    if (!asset?.asset_id) {
                      return <div className="absolute inset-0 bg-base-300" />;
                    }
                    return (
                      <img
                        src={assetUrls.getThumbnailUrl(asset.asset_id, "medium")}
                        alt={asset.original_filename || `featured-${index + 1}`}
                        className="h-full w-full object-cover"
                        loading="lazy"
                      />
                    );
                  }
                : undefined
            }
            onItemClick={(index) => {
              const asset = featuredAssets[index];
              if (!asset?.asset_id) return;
              navigate(`/assets/photos/${asset.asset_id}?groupBy=date`);
            }}
          />

          <div className="text-xs text-base-content/60 px-1">
            {isLoading
              ? "正在加载精选照片..."
              : featuredAssets.length > 0
                ? `featured: ${featuredAssets.length} / candidate: ${candidateCount} / seed: ${seed}`
                : "暂无可展示照片"}
          </div>

          <AICategoryCarousel />
          <FiltersCarousel />
        </>
      )}

      {displayMode === "stats" && (
        <div className="space-y-8 animate-fadeIn">
          <StatsCards />
          <InfoCard />
        </div>
      )}

      <SpacetimeMapCard
        assets={featuredAssets}
        subtitle={
          featuredAssets.length > 0
            ? `本次精选 ${featuredAssets.length} 张（seed: ${seed}）`
            : "暂无精选照片可展示"
        }
      />
    </div>
  );
}

export default Home;

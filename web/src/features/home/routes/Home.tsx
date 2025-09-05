import { useState } from "react";
import { SparklesIcon, CameraIcon } from "@heroicons/react/24/outline";
import GalleryGrid from "../components/GalleryGrid";
import AICategoryCarousel from "../components/AICategoryCarousel";
import FiltersCarousel from "../components/FiltersCarousel";
import StatsCards from "../components/StatsCards";
import SpacetimeMapCard from "../components/SpacetimeMapCard";
import InfoCard from "../components/InfoCard";
import { useI18n } from "@/lib/i18n.tsx";

function Home() {
  const [displayMode, setDisplayMode] = useState("gallery");
  const { t } = useI18n();

  return (
    <div className="flex flex-col gap-8 p-6 relative">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">{t("routes.home")}</h1>
        {/* 玻璃拟态风格Tab切换 */}
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

      {/* 画廊模式内容 */}
      {displayMode === "gallery" && (
        <>
          <GalleryGrid />
          <AICategoryCarousel />
          <FiltersCarousel />
        </>
      )}

      {/* 统计模式内容 */}
      {displayMode === "stats" && (
        <div className="space-y-8 animate-fadeIn">
          <StatsCards />
          <InfoCard />
        </div>
      )}

      {/* 时空地图整合区（始终显示） */}
      <SpacetimeMapCard />
    </div>
  );
}

export default Home;

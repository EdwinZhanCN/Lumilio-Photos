import { useState, useEffect } from "react";
import { useLocation } from "react-router-dom";
import Photos from "./Photos";
import Videos from "./Videos";
import Audios from "./Audios";
import AssetsProvider from "@/contexts/FetchContext";
import ErrorBoundary from "@/ErrorBoundary";
import AssetTabs from "@/components/Assets/AssetTabs";
import { useAssetsPageState } from "@/hooks/page-hooks/useAssetsPageState";

const Assets = () => {
  const location = useLocation();
  const [activeTab, setActiveTab] = useState("photos");

  // Get carousel state from the assets page state hook
  const { isCarouselOpen } = useAssetsPageState();

  // Determine active tab based on URL path
  useEffect(() => {
    const path = location.pathname;
    if (path.includes("/videos")) {
      setActiveTab("videos");
    } else if (path.includes("/audios")) {
      setActiveTab("audios");
    } else {
      // Default to photos for /assets/ or /assets/photos
      setActiveTab("photos");
    }
  }, [location.pathname]);

  return (
    <ErrorBoundary>
      <AssetsProvider>
        <div className="p-4 w-full max-w-screen-lg mx-auto mb-20">
          <div className="pt-4">
            {activeTab === "photos" && <Photos />}
            {activeTab === "videos" && <Videos />}
            {activeTab === "audios" && <Audios />}
          </div>
        </div>
        <AssetTabs isCarouselOpen={isCarouselOpen} />
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;

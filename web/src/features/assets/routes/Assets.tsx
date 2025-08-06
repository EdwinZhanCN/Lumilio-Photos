import { useState, useEffect } from "react";
import { useLocation } from "react-router-dom";
import Photos from "./Photos";
import Audios from "./Audios";
import Videos from "./Videos";
import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import AssetTabs from "@/features/assets/components/AssetTabs";
import { useAssetsPageState } from "@/features/assets/hooks/useAssetsPageState";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";

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
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title="Something went wrong" {...props} />
      )}
    >
      <AssetsProvider>
        <WorkerProvider preload={["exif", "export"]}>
          <div className="p-4 w-full mx-auto mb-20">
            <div className="pt-4">
              {activeTab === "photos" && <Photos />}
              {activeTab === "videos" && <Videos />}
              {activeTab === "audios" && <Audios />}
            </div>
          </div>
          <AssetTabs isCarouselOpen={isCarouselOpen} />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;

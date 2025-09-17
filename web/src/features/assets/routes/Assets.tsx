import { useState, useEffect } from "react";
import { useLocation } from "react-router-dom";
import Photos from "./Photos";
import Audios from "./Audios";
import Videos from "./Videos";
import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import AssetTabs from "@/features/assets/components/AssetTabs";
import { AssetsPageProvider, useAssetsPageContext } from "@/features/assets";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";

const AssetsContent = ({ activeTab }: { activeTab: string }) => {
  const { state } = useAssetsPageContext();
  const { isCarouselOpen } = state;

  return (
    <WorkerProvider preload={["exif", "export"]}>
      {activeTab === "photos" && <Photos />}
      {activeTab === "videos" && <Videos />}
      {activeTab === "audios" && <Audios />}
      <AssetTabs isCarouselOpen={isCarouselOpen} />
    </WorkerProvider>
  );
};

const Assets = () => {
  const location = useLocation();
  const [activeTab, setActiveTab] = useState("photos");

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
        <AssetsPageProvider>
          <AssetsContent activeTab={activeTab} />
        </AssetsPageProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;

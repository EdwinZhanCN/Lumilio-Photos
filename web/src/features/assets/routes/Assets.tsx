import { useMemo } from "react";
import { useLocation } from "react-router-dom";
import Photos from "./Photos";
import Audios from "./Audios";
import Videos from "./Videos";
import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import AssetTabs from "@/features/assets/components/AssetTabs";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";
import { useI18n } from "@/lib/i18n";
import { useIsCarouselOpen } from "@/features/assets/selectors";

const AssetsContent = ({ activeTab }: { activeTab: string }) => {
  const isCarouselOpen = useIsCarouselOpen();

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
  const { t } = useI18n();

  const activeTab = useMemo(() => {
    const path = location.pathname;
    if (path.includes("/videos")) {
      return "videos";
    }
    if (path.includes("/audios")) {
      return "audios";
    }
    return "photos";
  }, [location.pathname]);

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title={t("assets.errorFallback.something_went_wrong")} {...props} />
      )}
    >
      <AssetsProvider scopeId="assets:main" persist>
        <AssetsContent activeTab={activeTab} />
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;

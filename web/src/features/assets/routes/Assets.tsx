import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";
import { useI18n } from "@/lib/i18n";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";

const Assets = () => {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title={t("assets.errorFallback.something_went_wrong")} {...props} />
      )}
    >
      <AssetsProvider scopeId="assets:main" persist>
        <WorkerProvider preload={["exif", "export"]}>
          <AssetsGalleryPage category="all" />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;

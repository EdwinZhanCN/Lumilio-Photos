import PreviewUploadSection from "@/components/UploadAssets/PreviewUploadSection";
import BatchUploadSection from "@/components/UploadAssets/BatchUploadSection.tsx";
import UploadProvider from "@/contexts/UploadContext.tsx";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/pages/ErrorFallBack.tsx";
import { WorkerProvider } from "@/contexts/WorkerProvider";

const UploadPhotos = () => {
  return (
    <WorkerProvider preload={["hash", "thumbnail"]}>
      <UploadProvider>
        <div className="min-h-screen px-2">
          <ErrorBoundary
            FallbackComponent={(props) => (
              <ErrorFallBack
                code={"500"}
                title={"Preview Upload Section Error!"}
                message={
                  "The preview upload section failed to load or encountered an error during operation."
                }
                {...props}
              />
            )}
          >
            <PreviewUploadSection />
          </ErrorBoundary>
          <ErrorBoundary
            FallbackComponent={(props) => (
              <ErrorFallBack
                code={"500"}
                title={"Batch Upload Section Error!"}
                message={
                  "The batch upload section failed to load or encountered an error during operation."
                }
                {...props}
              />
            )}
          >
            <BatchUploadSection />
          </ErrorBoundary>
        </div>
      </UploadProvider>
    </WorkerProvider>
  );
};

export default UploadPhotos;

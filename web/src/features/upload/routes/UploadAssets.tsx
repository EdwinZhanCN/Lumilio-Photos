import PreviewUploadSection from "../components/PreviewUploadSection";
import BatchUploadSection from "../components/BatchUploadSection";
import { UploadProvider } from "../UploadProvider";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";

const UploadAssets = () => {
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

export default UploadAssets;

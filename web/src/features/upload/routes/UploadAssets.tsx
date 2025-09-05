import PreviewUploadSection from "../components/PreviewUploadSection";
import BatchUploadSection from "../components/BatchUploadSection";
import { UploadProvider } from "../UploadProvider";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PageHeader from "@/components/PageHeader";
import { useUploadContext } from "@/features/upload";
import React, { useRef } from "react";
import { ArrowUpTrayIcon } from "@heroicons/react/24/outline";

const UploadHeader: React.FC = () => {
  const {
    state,
    isProcessing,
    uploadAllFiles,
    clearAllFiles,
    uploadProgress,
    hashcodeProgress,
    isGeneratingHashCodes,
  } = useUploadContext();
  const clearRef = useRef<HTMLInputElement | null>(null);

  const total = state.totalFilesCount;
  const subtitle =
    isGeneratingHashCodes && hashcodeProgress?.total
      ? `Hashing ${hashcodeProgress.numberProcessed ?? 0}/${hashcodeProgress.total}`
      : isProcessing && uploadProgress > 0
        ? `Uploading ${uploadProgress.toFixed(1)}%`
        : total > 0
          ? `${total} files selected (${state.preview.count} preview, ${state.batch.count} batch)`
          : "No files selected";

  return (
    <PageHeader
      title="Upload Assets"
      icon={<ArrowUpTrayIcon className="w-6 h-6 text-primary" />}
    >
      <button
        className="btn btn-sm btn-ghost"
        onClick={() => clearAllFiles(clearRef)}
        disabled={isProcessing || total === 0}
      >
        Clear All
      </button>
      <button
        className="btn btn-sm btn-primary"
        onClick={() => uploadAllFiles()}
        disabled={isProcessing || total === 0}
      >
        {isProcessing ? "Processing..." : "Upload All"}
      </button>
      <small>{subtitle}</small>
    </PageHeader>
  );
};

const UploadAssets = () => {
  return (
    <WorkerProvider preload={["hash", "thumbnail"]}>
      <UploadProvider>
        <div className="min-h-screen">
          <UploadHeader />
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

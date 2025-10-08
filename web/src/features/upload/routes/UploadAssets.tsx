import UnifiedUploadSection from "../components/UnifiedUploadSection";
import { UploadProvider } from "../UploadProvider";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PageHeader from "@/components/PageHeader";
import { useUploadContext } from "@/features/upload";
import React from "react";
import { ArrowUpTrayIcon } from "@heroicons/react/24/outline";

const UploadHeader: React.FC = () => {
  const {
    state,
    isProcessing,
    uploadFiles,
    clearFiles,
    uploadProgress,
    hashcodeProgress,
    isGeneratingHashCodes,
    previewCount,
  } = useUploadContext();

  const total = state.files.length;
  const subtitle =
    isGeneratingHashCodes && hashcodeProgress?.total
      ? `Hashing ${hashcodeProgress.numberProcessed ?? 0}/${hashcodeProgress.total}`
      : isProcessing && uploadProgress > 0
        ? `Uploading ${uploadProgress.toFixed(1)}%`
        : total > 0
          ? `${total} files selected (${previewCount} with previews)`
          : "No files selected";

  return (
    <PageHeader
      title="Upload Assets"
      icon={<ArrowUpTrayIcon className="w-6 h-6 text-primary" />}
    >
      <button
        className="btn btn-sm btn-ghost"
        onClick={() => {
          clearFiles();
        }}
        disabled={isProcessing || total === 0}
      >
        Clear All
      </button>
      <button
        className="btn btn-sm btn-primary"
        onClick={() => uploadFiles()}
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
                title={"Upload Section Error!"}
                message={
                  "The upload section failed to load or encountered an error during operation."
                }
                {...props}
              />
            )}
          >
            <UnifiedUploadSection />
          </ErrorBoundary>
        </div>
      </UploadProvider>
    </WorkerProvider>
  );
};

export default UploadAssets;

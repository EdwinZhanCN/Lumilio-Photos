import UnifiedUploadSection from "../components/UnifiedUploadSection";
import { UploadProvider } from "../UploadProvider";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PageHeader from "@/components/PageHeader";
import { useUploadContext } from "@/features/upload";
import React, { useState } from "react";
import { ArrowUpTrayIcon, DocumentTextIcon } from "@heroicons/react/24/outline";
import SupportedFormatsModal from "../components/SupportedFormatsModal";

const UploadHeader: React.FC = () => {
  const { state, isProcessing } = useUploadContext();
  const [showFormatsModal, setShowFormatsModal] = useState(false);

  const total = state.files.length;
  const subtitle =
    isProcessing && total > 0
      ? `Processing ${total} file${total !== 1 ? "s" : ""}...`
      : total > 0
        ? `${total} file${total !== 1 ? "s" : ""} ready to upload`
        : "No files selected";

  return (
    <>
      <PageHeader
        title="Upload Assets"
        icon={<ArrowUpTrayIcon className="w-6 h-6 text-primary" />}
      >
        <button
          className="btn btn-sm btn-soft btn-info"
          onClick={() => setShowFormatsModal(true)}
        >
          <DocumentTextIcon className="w-4 h-4" />
          Supported Formats
        </button>
        <small>{subtitle}</small>
      </PageHeader>

      <SupportedFormatsModal
        isOpen={showFormatsModal}
        onClose={() => setShowFormatsModal(false)}
      />
    </>
  );
};

const UploadAssets = () => {
  return (
    <WorkerProvider preload={["hash"]}>
      <UploadProvider>
        <div className="min-h-screen bg-base-100">
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

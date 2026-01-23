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
import { useI18n } from "@/lib/i18n"; // Import useI18n

const UploadHeader: React.FC = () => {
  const { t } = useI18n(); // Initialize useI18n
  const { state, isProcessing } = useUploadContext();
  const [showFormatsModal, setShowFormatsModal] = useState(false);

  const total = state.files.length;
  const subtitle =
    isProcessing && total > 0
      ? t('upload.UploadAssets.processing_files', { count: total })
      : total > 0
        ? t('upload.UploadAssets.files_ready_to_upload', { count: total })
        : t('upload.UploadAssets.no_files_selected');

  return (
    <>
      <PageHeader
        title={t('upload.UploadAssets.page_title')}
        icon={<ArrowUpTrayIcon className="w-6 h-6 text-primary" />}
      >
        <button
          className="btn btn-sm btn-soft btn-info"
          onClick={() => setShowFormatsModal(true)}
        >
          <DocumentTextIcon className="w-4 h-4" />
          {t('upload.UploadAssets.supported_formats_button')}
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
  const { t } = useI18n(); // Initialize useI18n
  return (
    <WorkerProvider preload={["hash"]}>
      <UploadProvider>
        <div className="min-h-screen bg-base-100">
          <UploadHeader />
          <ErrorBoundary
            FallbackComponent={(props) => (
              <ErrorFallBack
                code={"500"}
                title={t('upload.UploadAssets.error_boundary_title')}
                message={t('upload.UploadAssets.error_boundary_message')}
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

import React, { useState } from "react";
import { FileTextIcon, Folders } from "lucide-react";
import { ErrorBoundary } from "react-error-boundary";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n";
import { useUploadContext } from "@/features/upload";
import UnifiedUploadSection from "@/features/upload/components/UnifiedUploadSection";
import SupportedFormatsModal from "@/features/upload/components/SupportedFormatsModal";
import RepositoryGrid from "@/features/manage/components/RepositoryGrid";

const ManageHeader: React.FC = () => {
  const { t } = useI18n();
  const { state, isProcessing } = useUploadContext();
  const [showFormatsModal, setShowFormatsModal] = useState(false);

  const total = state.files.length;
  const subtitle =
    isProcessing && total > 0
      ? t("upload.UploadAssets.processing_files", { count: total })
      : total > 0
        ? t("upload.UploadAssets.files_ready_to_upload", { count: total })
        : t("upload.UploadAssets.no_files_selected");

  return (
    <>
      <PageHeader
        title={t("manage.pageTitle")}
        subtitle={subtitle}
        icon={<Folders className="h-6 w-6 text-primary" />}
      >
        <button
          type="button"
          className="btn btn-sm btn-soft btn-info"
          onClick={() => setShowFormatsModal(true)}
        >
          <FileTextIcon className="h-4 w-4" />
          {t("upload.UploadAssets.supported_formats_button")}
        </button>
      </PageHeader>

      <SupportedFormatsModal
        isOpen={showFormatsModal}
        onClose={() => setShowFormatsModal(false)}
      />
    </>
  );
};

const Manage = () => {
  const { t } = useI18n();

  return (
    <div className="min-h-screen bg-base-100">
      <ManageHeader />
      <ErrorBoundary
        FallbackComponent={(props) => (
          <ErrorFallBack
            code="500"
            title={t("manage.errorBoundaryTitle")}
            message={t("manage.errorBoundaryMessage")}
            {...props}
          />
        )}
      >
        <UnifiedUploadSection />
        <RepositoryGrid />
      </ErrorBoundary>
    </div>
  );
};

export default Manage;

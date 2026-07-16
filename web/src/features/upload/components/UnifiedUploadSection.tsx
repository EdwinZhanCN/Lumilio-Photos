import React, { useRef, ChangeEvent, useMemo } from "react";
import FileDropZone from "./FileDropZone";

import { useUploadContext } from "../hooks/useUpload";
import { useUploadConfig } from "@/features/upload/hooks/useUploadQueries";
import { validateFile, getValidationErrorMessage } from "@/lib/utils/validate-file.ts";
import { getAcceptString } from "@/lib/utils/accept-file-extensions.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { Upload, Info, FolderPlus, FolderUp, X } from "lucide-react";
import { useI18n } from "@/lib/i18n"; // Import useI18n
import { useWorkingRepository } from "@/features/settings";

function UnifiedUploadSection(): React.JSX.Element {
  const { t } = useI18n(); // Initialize useI18n
  const { state, addFiles, clearFiles, uploadFiles, isProcessing } = useUploadContext();

  const { files } = state;
  const fileCount = files.length;

  const showMessage = useMessage();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const uploadConfigQuery = useUploadConfig();

  const uploadConfig = uploadConfigQuery.data;
  // This picker is the only surface for the working repository: the upload
  // target is chosen at the moment it takes effect, nowhere else.
  const {
    repositories,
    repositoriesQuery,
    workingRepositoryId,
    selectedRepository,
    setWorkingRepositoryId,
    getRepositoryLabel,
  } = useWorkingRepository();
  const primaryRepository = useMemo(
    () => repositories.find((repository) => repository.isPrimary),
    [repositories],
  );
  const uploadTargetRepository = selectedRepository ?? primaryRepository;
  const uploadTargetDescription = uploadTargetRepository?.path
    ? uploadTargetRepository.path
    : t("upload.UnifiedUploadSection.default_upload_target_hint");

  const formatBytes = (value?: number) => {
    if (!value && value !== 0) return "-";
    const mb = value / (1024 * 1024);
    return `${mb.toFixed(1)} MB`;
  };

  /**
   * Handle file selection and validation.
   */
  const handleFiles = async (selectedFiles: FileList) => {
    if (!selectedFiles || selectedFiles.length === 0) {
      return;
    }

    const validFiles = Array.from(selectedFiles).filter((file) => {
      const result = validateFile(file);
      if (!result.valid) {
        const errorMsg = getValidationErrorMessage(result);
        showMessage("error", `${file.name}: ${errorMsg}`);
      }
      return result.valid;
    });

    if (validFiles.length === 0) {
      showMessage("error", t("upload.UnifiedUploadSection.no_valid_files_selected"));
      return;
    }

    await addFiles(validFiles);
  };

  const handleClear = () => {
    if (isProcessing) {
      showMessage("error", t("upload.UnifiedUploadSection.cannot_clear_while_processing"));
      return;
    }
    clearFiles();
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleUpload = async () => {
    if (fileCount === 0) {
      showMessage("info", t("upload.UnifiedUploadSection.no_files_selected_for_upload"));
      return;
    }
    await uploadFiles();
  };

  return (
    <section id="unified-upload-section" className="container mx-auto px-4 py-8 max-w-5xl">
      {/* Main Drop Zone with integrated info */}
      <div className="relative">
        <FileDropZone fileInputRef={fileInputRef} onFilesDropped={(files) => handleFiles(files)}>
          <div className="space-y-4">
            <Upload className="mx-auto h-16 w-16 text-base-content/30" />
            <div>
              <p className="text-xl font-medium text-base-content/80">
                {t("upload.UnifiedUploadSection.drag_drop_or_click")}
              </p>
              <p className="text-sm text-base-content/50 mt-2">
                {t("upload.UnifiedUploadSection.supported_file_types_description")}
              </p>
            </div>
          </div>
        </FileDropZone>

        {/* Info Button (Tooltip) */}
        <div className="absolute top-4 right-4">
          <div className="dropdown dropdown-end">
            <label tabIndex={0} className="btn btn-circle btn-ghost btn-sm">
              <Info className="w-5 h-5" />
            </label>
            <div
              tabIndex={0}
              className="dropdown-content z-10 card card-compact w-80 max-w-[calc(100vw-2rem)] p-4 shadow-lg bg-base-200 text-base-content"
            >
              <div className="space-y-2 text-sm">
                <h3 className="font-semibold text-base">
                  {t("upload.UnifiedUploadSection.upload_information_title")}
                </h3>
                {uploadConfig && (
                  <div className="mt-2 space-y-1 text-xs text-base-content/70">
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_chunk_size")}
                      </span>{" "}
                      {formatBytes(uploadConfig.chunk_size)}
                    </p>
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_concurrency")}
                      </span>{" "}
                      {uploadConfig.max_concurrent ?? "-"}
                    </p>
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_in_flight")}
                      </span>{" "}
                      {uploadConfig.max_in_flight_requests ?? "-"}
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="mt-4 flex flex-col gap-2 rounded-lg border border-base-300 bg-base-200/40 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <FolderUp size={18} />
          </div>
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <label
                className="text-sm font-medium text-base-content/70"
                htmlFor="upload-target-repository"
              >
                {t("upload.UnifiedUploadSection.upload_target_label")}
              </label>
              <select
                id="upload-target-repository"
                className="select select-bordered select-sm w-full max-w-56"
                value={workingRepositoryId}
                disabled={repositoriesQuery.isLoading || repositoriesQuery.isError}
                onChange={(event) => setWorkingRepositoryId(event.target.value || null)}
              >
                {repositories.map((repository) => (
                  <option key={repository.id} value={repository.id}>
                    {getRepositoryLabel(repository)}
                  </option>
                ))}
              </select>
              {repositoriesQuery.isError && (
                <span className="text-xs text-base-content/60">
                  {t("navbar.repository.unavailable", {
                    defaultValue: "Repository options unavailable",
                  })}
                </span>
              )}
            </div>
            <p
              className="mt-1 truncate text-xs text-base-content/55"
              title={uploadTargetDescription}
            >
              {uploadTargetDescription}
            </p>
          </div>
        </div>
      </div>

      {/* Hidden file input */}
      <input
        type="file"
        ref={fileInputRef}
        className="hidden"
        multiple
        accept={getAcceptString()}
        onChange={(e: ChangeEvent<HTMLInputElement>) => {
          if (e.target.files) {
            void handleFiles(e.target.files);
            e.target.value = "";
          }
        }}
      />

      {/* Control Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 mt-6">
        {/* Left side - Add Files */}
        <div className="flex items-center gap-3">
          <button
            onClick={() => fileInputRef.current?.click()}
            className="btn btn-outline btn-sm gap-2"
            disabled={isProcessing}
          >
            <FolderPlus className="w-4 h-4" />
            {t("upload.UnifiedUploadSection.add_files_button")}
          </button>
        </div>

        {/* Right side - Action Buttons */}
        <div className="flex items-center gap-2">
          <button
            onClick={handleClear}
            disabled={fileCount === 0 || isProcessing}
            className="btn btn-ghost btn-sm gap-2"
          >
            <X className="w-4 h-4" />
            {t("upload.UnifiedUploadSection.clear_button")}
          </button>
          <button
            onClick={handleUpload}
            className="btn btn-primary btn-sm gap-2"
            disabled={fileCount === 0 || isProcessing}
          >
            <Upload className="w-4 h-4" />
            {isProcessing
              ? t("upload.UnifiedUploadSection.uploading_status")
              : t("upload.UnifiedUploadSection.upload_button", {
                  countLabel: fileCount > 0 ? ` (${fileCount})` : "",
                })}
          </button>
        </div>
      </div>

      {/* Custom scrollbar styles */}
      <style>{`
        .custom-scrollbar::-webkit-scrollbar {
          width: 6px;
        }
        .custom-scrollbar::-webkit-scrollbar-track {
          background: transparent;
        }
        .custom-scrollbar::-webkit-scrollbar-thumb {
          background: hsl(var(--bc) / 0.2);
          border-radius: 3px;
        }
        .custom-scrollbar::-webkit-scrollbar-thumb:hover {
          background: hsl(var(--bc) / 0.3);
        }
      `}</style>
    </section>
  );
}

export default UnifiedUploadSection;

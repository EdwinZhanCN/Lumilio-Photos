import React, { useRef, useEffect, ChangeEvent, useMemo } from "react";
import FileDropZone from "./FileDropZone";
import FileUploadProgress from "./FileUploadProgress";

import { useUploadContext } from "@/features/upload";
import {
  useUploadConfig,
  useUploadProgress,
} from "@/features/upload/hooks/useUploadQueries";
import {
  validateFile,
  getValidationErrorMessage,
} from "@/lib/utils/validate-file.ts";
import { getAcceptString } from "@/lib/utils/accept-file-extensions.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import {
  ArrowUpTrayIcon,
  XMarkIcon,
  InformationCircleIcon,
  FolderPlusIcon,
}
from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n"; // Import useI18n

function UnifiedUploadSection(): React.JSX.Element {
  const { t } = useI18n(); // Initialize useI18n
  const {
    state,
    addFiles,
    clearFiles,
    uploadFiles,
    uploadProgress,
    isProcessing,
    fileProgress,
    maxTotalFiles,
  } = useUploadContext();

  const { files } = state;
  const fileCount = files.length;

  const showMessage = useMessage();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const activeSessionIds = useMemo(() => {
    const ids = fileProgress
      .filter((file) => file.status === "uploading" && file.sessionId)
      .map((file) => file.sessionId);
    return Array.from(new Set(ids));
  }, [fileProgress]);

  const sessionIds = activeSessionIds.length > 0 ? activeSessionIds.join(",") : undefined;
  const uploadConfigQuery = useUploadConfig();
  const uploadProgressQuery = useUploadProgress(sessionIds, {
    enabled: activeSessionIds.length > 0,
    refetchInterval: activeSessionIds.length > 0 ? 2000 : false,
  });

  const uploadConfig = uploadConfigQuery.data?.data;
  const serverProgress = uploadProgressQuery.data?.data;
  const serverSummary = serverProgress?.summary;
  const serverSessions = serverProgress?.sessions ?? [];

  const formatBytes = (value?: number) => {
    if (!value && value !== 0) return "-";
    const mb = value / (1024 * 1024);
    return `${mb.toFixed(1)} MB`;
  };

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

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
      showMessage("error", t('upload.UnifiedUploadSection.no_valid_files_selected'));
      return;
    }

    await addFiles(validFiles);
  };

  const handleClear = () => {
    if (isProcessing) {
      showMessage("error", t('upload.UnifiedUploadSection.cannot_clear_while_processing'));
      return;
    }
    clearFiles();
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleUpload = async () => {
    if (fileCount === 0) {
      showMessage("info", t('upload.UnifiedUploadSection.no_files_selected_for_upload'));
      return;
    }
    await uploadFiles();
  };

  return (
    <section
      id="unified-upload-section"
      className="container mx-auto px-4 py-8 max-w-5xl"
    >
      {/* Main Drop Zone with integrated info */}
      <div className="relative">
        <FileDropZone
          fileInputRef={fileInputRef}
          onFilesDropped={(files) => handleFiles(files)}
        >
          <div className="space-y-4">
            <ArrowUpTrayIcon className="mx-auto h-16 w-16 text-base-content/30" />
            <div>
              <p className="text-xl font-medium text-base-content/80">
                {t('upload.UnifiedUploadSection.drag_drop_or_click')}
              </p>
              <p className="text-sm text-base-content/50 mt-2">
                {t('upload.UnifiedUploadSection.supported_file_types_description')}
              </p>
            </div>
          </div>
        </FileDropZone>

        {/* Info Button (Tooltip) */}
        <div className="absolute top-4 right-4">
          <div className="dropdown dropdown-end">
            <label tabIndex={0} className="btn btn-circle btn-ghost btn-sm">
              <InformationCircleIcon className="w-5 h-5" />
            </label>
            <div
              tabIndex={0}
              className="dropdown-content z-10 card card-compact w-80 p-4 shadow-lg bg-base-200 text-base-content"
            >
              <div className="space-y-2 text-sm">
                <h3 className="font-semibold text-base">{t('upload.UnifiedUploadSection.upload_information_title')}</h3>
                <p>
                  <span className="font-medium">{t('upload.UnifiedUploadSection.max_files_label')}</span>{" "}
                  {maxTotalFiles}
                </p>
                {uploadConfig && (
                  <div className="mt-2 space-y-1 text-xs text-base-content/70">
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_chunk_size", {
                          defaultValue: "Server chunk size",
                        })}
                      </span>{" "}
                      {formatBytes(uploadConfig.chunk_size)}
                    </p>
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_concurrency", {
                          defaultValue: "Server concurrency",
                        })}
                      </span>{" "}
                      {uploadConfig.max_concurrent ?? "-"}
                    </p>
                    <p>
                      <span className="font-medium">
                        {t("upload.UnifiedUploadSection.server_in_flight", {
                          defaultValue: "In-flight requests",
                        })}
                      </span>{" "}
                      {uploadConfig.max_in_flight_requests ?? "-"}
                    </p>
                  </div>
                )}
                <p className="text-base-content/70 text-xs mt-2">
                  {t('upload.UnifiedUploadSection.change_limits_settings_hint')}
                </p>
              </div>
            </div>
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
            handleFiles(e.target.files);
            e.target.value = "";
          }
        }}
      />

      {/* Control Bar */}
      <div className="flex items-center justify-between gap-4 mt-6">
        {/* Left side - Add Files */}
        <div className="flex items-center gap-3">
          <button
            onClick={() => fileInputRef.current?.click()}
            className="btn btn-outline btn-sm gap-2"
            disabled={isProcessing || fileCount >= maxTotalFiles}
          >
            <FolderPlusIcon className="w-4 h-4" />
            {t('upload.UnifiedUploadSection.add_files_button')}
          </button>
        </div>

        {/* Right side - Action Buttons */}
        <div className="flex items-center gap-2">
          <button
            onClick={handleClear}
            disabled={fileCount === 0 || isProcessing}
            className="btn btn-ghost btn-sm gap-2"
          >
            <XMarkIcon className="w-4 h-4" />
            {t('upload.UnifiedUploadSection.clear_button')}
          </button>
          <button
            onClick={handleUpload}
            className="btn btn-primary btn-sm gap-2"
            disabled={fileCount === 0 || isProcessing}
          >
            <ArrowUpTrayIcon className="w-4 h-4" />
            {isProcessing
              ? t('upload.UnifiedUploadSection.uploading_status')
              : t('upload.UnifiedUploadSection.upload_button', { count: fileCount })}
          </button>
        </div>
      </div>

      {/* File List - Floating Card Style */}
      {fileCount > 0 && (
        <div className="mt-6">
          <div className="card bg-base-200 shadow-xl">
            <div className="card-body p-4">
              <h3 className="card-title text-base">
                {t('upload.UnifiedUploadSection.selected_files_title')}
                <div className="badge badge-primary badge-sm">{fileCount}</div>
              </h3>

              {/* Progress bar */}
              <progress
                className="progress progress-primary w-full h-1"
                value={fileCount}
                max={maxTotalFiles}
              />

              {/* File list with smooth scrolling */}
              <div className="space-y-2 max-h-80 overflow-y-auto pr-2 custom-scrollbar">
                {files.map((file, index) => (
                  <div
                    key={index}
                    className="flex items-center justify-between p-3 bg-base-100 rounded-lg hover:bg-base-300 transition-colors"
                  >
                    <div className="flex items-center gap-3 flex-1 min-w-0">
                      {/* File icon */}
                      <div className="avatar placeholder">
                        <div className="bg-neutral text-neutral-content rounded w-10 h-10">
                          <span className="text-xs">
                            {file.name
                              .split(".")
                              .pop()
                              ?.toUpperCase()
                              .slice(0, 3)}
                          </span>
                        </div>
                      </div>

                      {/* File info */}
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">
                          {file.name}
                        </p>
                        <p className="text-xs text-base-content/60">
                          {(file.size / 1024 / 1024).toFixed(2)} MB
                        </p>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Individual File Upload Progress */}
      {fileProgress.length > 0 && (
        <div className="mt-6">
          <FileUploadProgress fileProgress={fileProgress} />
        </div>
      )}

      {/* Overall Upload Progress */}
      {uploadProgress > 0 && (
        <div className="mt-6 card bg-base-200 shadow-lg">
          <div className="card-body p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium">{t('upload.UnifiedUploadSection.overall_progress_label')}</span>
              <span className="text-sm font-bold text-primary">
                {uploadProgress.toFixed(1)}%
              </span>
            </div>
            <progress
              className="progress progress-primary w-full"
              value={uploadProgress}
              max="100"
            />
          </div>
        </div>
      )}

      {(serverSummary || serverSessions.length > 0) && (
        <div className="mt-6 card bg-base-200 shadow-lg">
          <div className="card-body p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium">
                {t("upload.UnifiedUploadSection.server_progress_label", {
                  defaultValue: "Server processing",
                })}
              </span>
              {typeof serverSummary?.overall_progress === "number" && (
                <span className="text-sm font-bold text-primary">
                  {serverSummary.overall_progress.toFixed(1)}%
                </span>
              )}
            </div>

            {typeof serverSummary?.overall_progress === "number" && (
              <progress
                className="progress progress-primary w-full"
                value={serverSummary.overall_progress}
                max="100"
              />
            )}

            {serverSummary && (
              <div className="mt-2 flex flex-wrap gap-4 text-xs text-base-content/70">
                <span>
                  {t("upload.UnifiedUploadSection.server_active_sessions", {
                    defaultValue: "Active sessions",
                  })}
                  : {serverSummary.active_sessions ?? 0}
                </span>
                <span>
                  {t("upload.UnifiedUploadSection.server_completed_files", {
                    defaultValue: "Completed files",
                  })}
                  : {serverSummary.completed_files ?? 0}
                </span>
                <span>
                  {t("upload.UnifiedUploadSection.server_failed_sessions", {
                    defaultValue: "Failed sessions",
                  })}
                  : {serverSummary.failed_sessions ?? 0}
                </span>
              </div>
            )}

            {serverSessions.length > 0 && (
              <div className="mt-3 space-y-2 max-h-40 overflow-y-auto pr-2 custom-scrollbar">
                {serverSessions.map((session, index) => (
                  <div
                    key={session.session_id || index}
                    className="rounded bg-base-100 px-3 py-2"
                  >
                    <div className="flex items-center justify-between text-xs">
                      <span className="truncate max-w-[70%]">
                        {session.filename || session.session_id}
                      </span>
                      <span className="font-medium">
                        {typeof session.progress === "number"
                          ? `${session.progress.toFixed(1)}%`
                          : "-"}
                      </span>
                    </div>
                    <progress
                      className="progress progress-primary w-full mt-1"
                      value={session.progress ?? 0}
                      max="100"
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

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

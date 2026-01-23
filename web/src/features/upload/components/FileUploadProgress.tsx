import React from "react";
import type { FileUploadProgress } from "@/hooks/api-hooks/useUploadProcess";
import {
  CheckCircleIcon,
  XCircleIcon,
  ArrowPathIcon,
  ClockIcon,
} from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n"; // Import useI18n

interface FileUploadProgressProps {
  fileProgress: FileUploadProgress[];
}

const FileUploadProgress: React.FC<FileUploadProgressProps> = ({
  fileProgress,
}) => {
  const { t } = useI18n(); // Initialize useI18n

  if (fileProgress.length === 0) {
    return null;
  }

  const getStatusColor = (status: FileUploadProgress["status"]) => {
    switch (status) {
      case "completed":
        return "text-success";
      case "uploading":
        return "text-primary";
      case "failed":
        return "text-error";
      case "pending":
      default:
        return "text-base-content/50";
    }
  };

  const getStatusIcon = (status: FileUploadProgress["status"]) => {
    switch (status) {
      case "completed":
        return <CheckCircleIcon className="w-5 h-5 text-success" />;
      case "uploading":
        return <ArrowPathIcon className="w-5 h-5 text-primary animate-spin" />;
      case "failed":
        return <XCircleIcon className="w-5 h-5 text-error" />;
      case "pending":
      default:
        return <ClockIcon className="w-5 h-5 text-base-content/50" />;
    }
  };

  const getProgressBarColor = (status: FileUploadProgress["status"]) => {
    switch (status) {
      case "completed":
        return "progress-success";
      case "uploading":
        return "progress-primary";
      case "failed":
        return "progress-error";
      case "pending":
      default:
        return "progress-base-300";
    }
  };

  const getStatusText = (status: FileUploadProgress["status"]) => {
    switch (status) {
      case "completed":
        return t('upload.FileUploadProgress.status_completed');
      case "uploading":
        return t('upload.FileUploadProgress.status_uploading');
      case "failed":
        return t('upload.FileUploadProgress.status_failed');
      case "pending":
      default:
        return t('upload.FileUploadProgress.status_pending');
    }
  };

  return (
    <div className="card bg-base-200 shadow-lg">
      <div className="card-body p-4">
        <h3 className="card-title text-base mb-2">
          {t('upload.FileUploadProgress.title')}
          <div className="badge badge-primary badge-sm">
            {fileProgress.length}
          </div>
        </h3>

        <div className="space-y-3 max-h-96 overflow-y-auto pr-2 custom-scrollbar">
          {fileProgress.map((file, index) => (
            <div
              key={index}
              className="p-3 bg-base-100 rounded-lg hover:bg-base-300/50 transition-colors"
            >
              {/* Header */}
              <div className="flex items-center gap-3 mb-2">
                {/* Status Icon */}
                <div className="flex-shrink-0">
                  {getStatusIcon(file.status)}
                </div>

                {/* File Info */}
                <div className="flex-1 min-w-0">
                  <p
                    className="text-sm font-medium truncate"
                    title={file.fileName}
                  >
                    {file.fileName}
                  </p>
                  <div className="flex items-center gap-2 mt-1">
                    <span className={`text-xs ${getStatusColor(file.status)}`}>
                      {getStatusText(file.status)}
                    </span>
                    {file.isChunked && (
                      <span className="badge badge-xs badge-outline">
                        {t('upload.FileUploadProgress.chunked_badge')}
                      </span>
                    )}
                  </div>
                </div>

                {/* Progress Percentage */}
                <div className="flex-shrink-0">
                  <div
                    className={`text-sm font-bold ${getStatusColor(file.status)}`}
                  >
                    {file.progress.toFixed(0)}%
                  </div>
                </div>
              </div>

              {/* Progress Bar */}
              <progress
                className={`progress w-full h-2 ${getProgressBarColor(file.status)}`}
                value={file.progress}
                max="100"
              />

              {/* Error Message */}
              {file.error && (
                <div className="mt-2 alert alert-error py-2 px-3">
                  <XCircleIcon className="w-4 h-4 flex-shrink-0" />
                  <span className="text-xs">{file.error}</span>
                </div>
              )}
            </div>
          ))}
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
    </div>
  );
};

export default FileUploadProgress;

import React from 'react';
import type { FileUploadProgress } from '@/hooks/api-hooks/useUploadProcess';

interface FileUploadProgressProps {
  fileProgress: FileUploadProgress[];
}

const FileUploadProgress: React.FC<FileUploadProgressProps> = ({ fileProgress }) => {
  if (fileProgress.length === 0) {
    return null;
  }

  const getStatusColor = (status: FileUploadProgress['status']) => {
    switch (status) {
      case 'completed':
        return 'text-success';
      case 'uploading':
        return 'text-primary';
      case 'failed':
        return 'text-error';
      case 'pending':
      default:
        return 'text-base-content/70';
    }
  };

  const getStatusIcon = (status: FileUploadProgress['status']) => {
    switch (status) {
      case 'completed':
        return '✓';
      case 'uploading':
        return '↻';
      case 'failed':
        return '✗';
      case 'pending':
      default:
        return '⋯';
    }
  };

  const getProgressBarColor = (status: FileUploadProgress['status']) => {
    switch (status) {
      case 'completed':
        return 'progress-success';
      case 'uploading':
        return 'progress-primary';
      case 'failed':
        return 'progress-error';
      case 'pending':
      default:
        return 'progress-base-300';
    }
  };

  return (
    <div className="mt-6 p-4 bg-base-200 rounded-lg">
      <h3 className="font-semibold mb-3">Upload Progress</h3>
      <div className="space-y-3 max-h-80 overflow-y-auto">
        {fileProgress.map((file, index) => (
          <div key={index} className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 flex-1 min-w-0">
                <span className={`text-sm font-medium ${getStatusColor(file.status)}`}>
                  {getStatusIcon(file.status)}
                </span>
                <span className="text-sm truncate flex-1" title={file.fileName}>
                  {file.fileName}
                </span>
                {file.isChunked && (
                  <span className="text-xs text-base-content/50 bg-base-300 px-2 py-1 rounded">
                    chunked
                  </span>
                )}
              </div>
              <span className="text-sm text-base-content/70 ml-2 whitespace-nowrap">
                {file.progress.toFixed(0)}%
              </span>
            </div>

            <div className="flex items-center gap-2">
              <progress
                className={`progress w-full ${getProgressBarColor(file.status)}`}
                value={file.progress}
                max="100"
              />
            </div>

            {file.error && (
              <div className="text-xs text-error bg-error/10 px-2 py-1 rounded">
                {file.error}
              </div>
            )}

            {index < fileProgress.length - 1 && (
              <div className="border-t border-base-300 pt-2" />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default FileUploadProgress;

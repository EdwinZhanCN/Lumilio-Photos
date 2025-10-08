import React, { useRef, useEffect, ChangeEvent } from "react";
import FileDropZone from "./FileDropZone";
import ProgressIndicator from "./ProgressIndicator";
import ImagePreviewGrid from "./ImagePreviewGrid";
import { useUploadContext } from "@/features/upload";
import ValidateFile from "@/lib/utils/validate-file.ts";
import { acceptFileExtensions } from "@/lib/utils/accept-file-extensions.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";

function UnifiedUploadSection(): React.JSX.Element {
  const {
    state,
    addFiles,
    clearFiles,
    uploadFiles,
    uploadProgress,
    isProcessing,
    isGeneratingPreviews,
    previewProgress,
    maxPreviewCount,
    maxTotalFiles,
    previewCount,
  } = useUploadContext();

  const { files, previews } = state;
  const fileCount = files.length;

  const showMessage = useMessage();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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
  const handleFiles = async (selectedFiles: FileList, shouldGeneratePreviews: boolean = true) => {
    if (!selectedFiles || selectedFiles.length === 0) {
      return;
    }

    const validFiles = Array.from(selectedFiles).filter((file) => {
      const isValid = ValidateFile(file);
      if (!isValid) {
        showMessage("error", `Invalid file: ${file.name}`);
      }
      return isValid;
    });

    if (validFiles.length === 0) {
      showMessage("error", "No valid files selected");
      return;
    }

    await addFiles(validFiles, shouldGeneratePreviews);
  };

  const handleClear = () => {
    if (isProcessing) {
      showMessage("error", "Cannot clear files while processing");
      return;
    }
    clearFiles();
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleUpload = async () => {
    if (fileCount === 0) {
      showMessage("info", "No files selected for upload");
      return;
    }
    await uploadFiles();
  };

  return (
    <section id="unified-upload-section" className="max-w-3xl mx-auto">
      <h2 className="text-2xl font-semibold mb-4">Upload Assets</h2>

      <div className="mb-6 p-4 bg-base-200 rounded-lg">
        <div className="space-y-2 text-sm">
          <p>
            <span className="font-semibold">Total files:</span> {fileCount} / {maxTotalFiles}
          </p>
          <p>
            <span className="font-semibold">Files with previews:</span> {previewCount} / {maxPreviewCount}
          </p>
          <p className="text-base-content/70">
            The first {maxPreviewCount} files will have thumbnail previews generated.
            You can change these limits in Settings → UI Settings.
          </p>
        </div>
        <progress
          className="progress progress-primary w-full mt-3"
          value={fileCount}
          max={maxTotalFiles}
        />
      </div>

      {/* Drop Zone */}
      <FileDropZone
        fileInputRef={fileInputRef}
        onFilesDropped={(files) => handleFiles(files, true)}
      >
        <p className="font-medium">Drag or Click Here to Upload</p>
        <p className="text-sm">Supports JPEG, PNG, RAW, Video, and more</p>
      </FileDropZone>

      {/* Click Zone */}
      <input
        type="file"
        ref={fileInputRef}
        className="hidden"
        multiple
        accept={`image/*,video/*,${acceptFileExtensions.join(",")}`}
        onChange={(e: ChangeEvent<HTMLInputElement>) => {
          if (e.target.files) {
            handleFiles(e.target.files, true);
            e.target.value = "";
          }
        }}
      />

      {/* Quick upload buttons */}
      <div className="flex gap-4 mb-6">
        <button
          onClick={() => fileInputRef.current?.click()}
          className="btn btn-outline btn-sm"
          disabled={isProcessing || fileCount >= maxTotalFiles}
        >
          Add More Files
        </button>
        <button
          onClick={() => {
            const input = document.createElement("input");
            input.type = "file";
            input.multiple = true;
            input.accept = `image/*,video/*,${acceptFileExtensions.join(",")}`;
            input.onchange = (e) => {
              const target = e.target as HTMLInputElement;
              if (target.files) {
                handleFiles(target.files, false); // Don't generate previews
              }
            };
            input.click();
          }}
          className="btn btn-outline btn-sm"
          disabled={isProcessing || fileCount >= maxTotalFiles}
        >
          Add Files (No Preview)
        </button>
      </div>

      {/* Preview generation progress */}
      {isGeneratingPreviews && previewProgress && (
        <div className="mb-6">
          <div className="flex items-center gap-4">
            <ProgressIndicator
              processed={previewProgress.numberProcessed}
              total={previewProgress.total}
              label="Generating thumbnails"
            />
            <span className="loading loading-dots loading-md" />
          </div>
        </div>
      )}

      {/* Preview Grid */}
      {previews.length > 0 && (
        <>
          <h3 className="text-lg font-semibold mb-3">Preview</h3>
          <ImagePreviewGrid previews={previews} />
        </>
      )}

      {/* File List */}
      {fileCount > 0 && (
        <div className="mt-6 p-4 bg-base-200 rounded-lg">
          <h3 className="font-semibold mb-3">Selected Files ({fileCount})</h3>
          <div className="space-y-1 max-h-60 overflow-y-auto">
            {files.slice(0, 10).map((file, index) => (
              <div key={index} className="text-sm flex justify-between items-center">
                <span className="truncate flex-1">
                  {file.name}
                </span>
                <span className="text-base-content/70 ml-2">
                  {(file.size / 1024 / 1024).toFixed(2)} MB
                  {previews[index] && " (preview)"}
                </span>
              </div>
            ))}
            {fileCount > 10 && (
              <div className="text-sm text-base-content/70 pt-2 border-t border-base-300">
                ... and {fileCount - 10} more files
              </div>
            )}
          </div>
        </div>
      )}

      {/* Upload Progress */}
      {uploadProgress > 0 && (
        <div className="mt-4">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-sm font-medium">Upload Progress:</span>
            <span className="text-sm text-base-content/70">
              {uploadProgress.toFixed(1)}%
            </span>
          </div>
          <progress
            className="progress progress-success w-full"
            value={uploadProgress}
            max="100"
          >
            {uploadProgress}%
          </progress>
        </div>
      )}

      {/* Action Buttons */}
      <div className="flex justify-end gap-4 mt-6">
        <button
          onClick={handleClear}
          disabled={fileCount === 0 || isProcessing}
          className="btn btn-ghost"
        >
          Clear All
        </button>
        <button
          onClick={handleUpload}
          className="btn btn-primary"
          disabled={fileCount === 0 || isProcessing}
        >
          {isProcessing ? "Processing..." : `Upload ${fileCount} File${fileCount !== 1 ? 's' : ''}`}
        </button>
      </div>
    </section>
  );
}

export default UnifiedUploadSection;

import React, { useRef, useEffect, ChangeEvent } from "react";
import FileDropZone from "./FileDropZone";
import ProgressIndicator from "./ProgressIndicator";
import ImagePreviewGrid from "./ImagePreviewGrid";
import { useUploadContext } from "@/features/upload";
import { useGenerateThumbnail } from "@/hooks/util-hooks/useGenerateThumbnail";
import ValidateFile from "@/lib/utils/validate-file.ts";
import { acceptFileExtensions } from "@/lib/utils/accept-file-extensions.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";

function PreviewUploadSection(): React.JSX.Element {
  const {
    state,
    dispatch,
    uploadProgress,
    clearPreviewFiles,
    uploadPreviewFiles,
    isProcessing,
  } = useUploadContext();

  const {
    preview: { files: previewFiles, previews: previewPreviews, count },
    maxPreviewFiles,
  } = state;

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

  const { isGenerating, progress, generatePreviews } = useGenerateThumbnail();

  /**
   * Handle file selection and validation.
   */
  const handleFiles = (selectedFiles: FileList) => {
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

    const availableSlots = maxPreviewFiles - count;

    if (availableSlots <= 0) {
      showMessage(
        "hint",
        `You can only upload at most ${maxPreviewFiles} files`,
      );
      return;
    }

    const filteredFiles = validFiles.slice(0, availableSlots);
    if (validFiles.length > availableSlots) {
      showMessage(
        "error",
        `${validFiles.length - availableSlots} files exceeded the limit and were removed`,
      );
    }

    const startIndex = count;

    dispatch({
      type: "SET_PREVIEW_FILES",
      payload: {
        files: [...previewFiles, ...filteredFiles],
        previews: [...previewPreviews, ...filteredFiles.map(() => "")],
      },
    });

    // Generate thumbnails and update only the preview URLs
    generatePreviews(filteredFiles)
      .then((result) => {
        if (result instanceof Error) {
          throw result;
        }

        if (!result || !Array.isArray(result)) {
          throw new Error("Invalid thumbnail generation result");
        }

        const newPreviewUrls = result.map((p) => p?.url || "");

        dispatch({
          type: "UPDATE_PREVIEW_URLS",
          payload: {
            startIndex,
            urls: newPreviewUrls,
          },
        });
      })
      .catch((error) => {
        console.error("Thumbnail generation failed:", error);
        const errorMessage =
          error instanceof Error
            ? error.message
            : "Failed to generate previews";
        showMessage("error", errorMessage);

        // Remove the files that failed to generate previews
        dispatch({
          type: "SET_PREVIEW_FILES",
          payload: {
            files: previewFiles,
            previews: previewPreviews,
          },
        });
      });
  };

  const handleClear = () => {
    if (isProcessing) {
      showMessage("error", "Cannot clear files while processing");
      return;
    }
    clearPreviewFiles(fileInputRef);
  };

  return (
    <section id="preview-upload-assets" className="max-w-3xl mx-auto">
      <h1 className="text-3xl font-bold mb-8">Preview Upload Assets</h1>
      <small className="text-sm text-base-content/70">
        This Section is for previewing the photos you want to upload. <br />
        The maximum number of files you can upload is {maxPreviewFiles}. <br />
        You can change the maximum number of files in the system setting.
      </small>

      {/* Drop Zone */}
      <FileDropZone fileInputRef={fileInputRef} onFilesDropped={handleFiles} />

      {/* Click Zone */}
      <input
        type="file"
        ref={fileInputRef}
        className="hidden"
        multiple
        accept={`image/*,video/*,${acceptFileExtensions.join(",")}`}
        onChange={(e: ChangeEvent<HTMLInputElement>) => {
          if (e.target.files) {
            handleFiles(e.target.files);
            e.target.value = "";
          }
        }}
      />

      <div className="text-sm text-gray-500 mb-4">
        {count} / {maxPreviewFiles} files
        <progress
          className="ml-2 w-32 h-2 align-middle"
          value={count}
          max={maxPreviewFiles}
        />
      </div>

      {isGenerating && progress && (
        <div className="flex gap-4">
          <ProgressIndicator
            processed={progress.numberProcessed}
            total={progress.total}
          />
          <div className="flex justify-center items-center mb-6">
            <span className="loading loading-dots loading-md" />
          </div>
        </div>
      )}

      <ImagePreviewGrid previews={previewPreviews} />

      {uploadProgress > 0 && (
        <div className="mb-4">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-sm font-medium">Upload Progress:</span>
            <span className="text-sm text-gray-500">
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

      <div className="flex justify-end gap-4">
        <button
          onClick={handleClear}
          disabled={count === 0 || isProcessing}
          className="mb-2 mt-2 btn btn-ghost"
        >
          Clear
        </button>
        <button
          onClick={uploadPreviewFiles}
          className="mb-2 mt-2 btn btn-primary"
          disabled={count === 0 || isProcessing}
        >
          {isProcessing ? "Uploading..." : "Start Upload"}
        </button>
      </div>
    </section>
  );
}

export default PreviewUploadSection;

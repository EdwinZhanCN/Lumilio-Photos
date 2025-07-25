import React, { useRef, useEffect, ChangeEvent } from "react";
import FileDropZone from "@/components/UploadAssets/FileDropZone.tsx";
import ProgressIndicator from "@/components/UploadAssets/ProgressIndicator";
import ImagePreviewGrid from "@/components/UploadAssets/ImagePreviewGrid";
import { useUploadContext } from "@/contexts/UploadContext";
import { useGenerateThumbnail } from "@/hooks/util-hooks/useGenerateThumbnail";
import ValidateFile from "@/utils/validate-file";
import { acceptFileExtensions } from "@/utils/accept-file-extensions";
import { useMessage } from "@/hooks/util-hooks/useMessage";

function PreviewUploadSection(): React.JSX.Element {
  const {
    state,
    dispatch,
    uploadProgress,
    clearPreviewFiles,
    uploadPreviewFiles,
  } = useUploadContext();

  const { previewFiles, previewPreviews, maxPreviewFiles } = state;
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
    const validFiles = Array.from(selectedFiles).filter((file) =>
      ValidateFile(file),
    );
    const availableSlots = maxPreviewFiles - previewFiles.length;

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
        `The exceeded ${validFiles.length - availableSlots} files have been removed`,
      );
    }

    dispatch({
      type: "SET_PREVIEW_FILES",
      payload: {
        files: [...previewFiles, ...filteredFiles],
        previews: [...previewPreviews, ...filteredFiles.map(() => "")],
      },
    });

    // update returned thumbnails using dispatch
    generatePreviews(filteredFiles)
      .then((result) => {
        if (result instanceof Error) {
          throw result;
        }
        const newPreviewUrls = result?.map((p) => p.url);

        dispatch({
          type: "SET_PREVIEW_FILES",
          payload: {
            files: [...previewFiles, ...filteredFiles],
            previews: [...previewPreviews, ...(newPreviewUrls || [])],
          },
        });
      })
      .catch((error) => {
        console.error("Thumbnail generation failed:", error);
        showMessage("error", "Failed to generate previews");
        handleClear();
      });
  };

  const handleClear = () => {
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
      <FileDropZone
        fileInputRef={fileInputRef}
        onFilesDropped={handleFiles}
        children={undefined}
      />

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
        {previewFiles.length} / {maxPreviewFiles} files
        <progress
          className="ml-2 w-32 h-2 align-middle"
          value={previewFiles.length}
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
        <progress
          className="progress progress-success w-56"
          value={uploadProgress}
          max="100"
        >
          Upload Progress
        </progress>
      )}

      <div className="flex justify-end gap-4">
        <button
          onClick={handleClear}
          disabled={previewFiles.length === 0 || uploadProgress > 0}
          className="mb-2 mt-2 btn btn-ghost"
        >
          Clear
        </button>
        <button
          onClick={uploadPreviewFiles}
          className="mb-2 mt-2 btn btn-primary"
          disabled={previewFiles.length === 0 || uploadProgress > 0}
        >
          {uploadProgress > 0 ? "Uploading..." : "Start Upload"}
        </button>
      </div>
    </section>
  );
}

export default PreviewUploadSection;

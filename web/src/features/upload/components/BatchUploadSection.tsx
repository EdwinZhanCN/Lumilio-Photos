import { useRef } from "react";
import ProgressIndicator from "./ProgressIndicator";
import { acceptFileExtensions } from "@/lib/utils/accept-file-extensions.ts";
import { useUploadContext } from "@/features/upload";

function BatchUploadSection() {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const {
    state,
    uploadBatchFiles,
    isGeneratingHashCodes,
    isProcessing,
    hashcodeProgress,
    dispatch,
    clearBatchFiles,
  } = useUploadContext();

  const {
    batch: { files: batchFiles, count: batchFilesCount },
    maxBatchFiles,
  } = state;

  const handleFileChange = (uploadFiles: FileList) => {
    const filesArray = Array.from(uploadFiles);
    dispatch({
      type: "SET_BATCH_FILES",
      payload: {
        files: filesArray,
      },
    });
  };

  return (
    <section id="batch-upload-assets" className="max-w-3xl mx-auto">
      <h1 className="text-3xl font-bold">Batch Upload Assets</h1>
      <small className="text-sm">
        This Section is for large files like RAW, Video to upload. <br />
        Selected files: {batchFilesCount} / {maxBatchFiles} <br />
      </small>

      {isGeneratingHashCodes && hashcodeProgress && (
        <ProgressIndicator
          processed={hashcodeProgress.numberProcessed}
          total={hashcodeProgress.total}
          label="Generating hashes"
        />
      )}

      <button
        onClick={() => fileInputRef.current?.click()}
        className="mb-2 mt-4 btn btn-primary"
        disabled={isProcessing}
      >
        {isProcessing ? "Processing..." : "Select Batch Files"}
      </button>

      <input
        type="file"
        ref={fileInputRef}
        className="hidden"
        multiple
        accept={"image/*,video/*," + acceptFileExtensions.join(",")}
        onChange={(event) => {
          if (event.target.files) {
            handleFileChange(event.target.files);
          }
        }}
      />

      {batchFilesCount > 0 && (
        <div className="mt-4 p-4 bg-base-200 rounded-lg">
          <h3 className="font-semibold mb-2">Selected Batch Files:</h3>
          <div className="space-y-1">
            {batchFiles.slice(0, 5).map((file, index) => (
              <div key={index} className="text-sm">
                {file.name} ({(file.size / 1024 / 1024).toFixed(2)} MB)
              </div>
            ))}
            {batchFilesCount > 5 && (
              <div className="text-sm text-gray-500">
                ... and {batchFilesCount - 5} more files
              </div>
            )}
          </div>
          <div className="flex gap-2 mt-3">
            <button
              onClick={() => uploadBatchFiles()}
              className="btn btn-primary btn-sm"
              disabled={isProcessing}
            >
              {isGeneratingHashCodes
                ? "Generating Hashes..."
                : isProcessing
                  ? "Uploading..."
                  : "Upload Batch Files"}
            </button>
            <button
              onClick={() => clearBatchFiles(fileInputRef)}
              className="btn btn-ghost btn-sm"
              disabled={isProcessing}
            >
              Clear
            </button>
          </div>
        </div>
      )}
    </section>
  );
}

export default BatchUploadSection;

import { useRef } from "react";
import ProgressIndicator from "@/components/UploadAssets/ProgressIndicator";
import { acceptFileExtensions } from "@/utils/accept-file-extensions.ts";
import { useUploadContext } from "@/contexts/UploadContext.tsx";

function BatchUploadSection() {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  // 1. Updated destructuring from the context hook
  const { BatchUpload, isGeneratingHashCodes, isProcessing, hashcodeProgress } =
    useUploadContext();
  // This function correctly calls BatchUpload. No changes needed here.
  const handleFileChange = (uploadFiles: FileList) => {
    if (uploadFiles.length > 0) {
      BatchUpload(uploadFiles);
    }
  };

  return (
    <section
      id="batch-upload-assets"
      className="max-w-3xl mx-auto p-6 rounded-lg bg-white shadow"
    >
      <h1 className="text-3xl font-bold">Batch Upload Assets</h1>
      <small className="text-sm text-gray-500">
        This Section is for large files like RAW, Video to upload. <br />
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
        disabled={isGeneratingHashCodes || isProcessing}
      >
        {isGeneratingHashCodes
          ? "Generating Hashes..."
          : isProcessing
            ? "Uploading..."
            : "Batch Upload"}
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
    </section>
  );
}

export default BatchUploadSection;

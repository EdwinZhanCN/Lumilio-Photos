import { useRef } from "react";
import ProgressIndicator from "@/components/UploadAssets/ProgressIndicator";
import { acceptFileExtensions } from "@/utils/accept-file-extensions.ts";
import { useUploadContext } from "@/contexts/UploadContext.tsx";

function BatchUploadSection() {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  // TODO Handel file change

  const {
    BatchUpload,
    isGeneratingHashCodes,
    isProcessing,
    uploadProgress,
    hashcodeProgress,
    filesCount,
    setFilesCount,
  } = useUploadContext();

  const handleFileChange = (uploadFiles: FileList) => {
    if (uploadFiles.length > 0) {
      BatchUpload(uploadFiles);
    }
  };

  return (
    <section id="batch-upload-assets" className="max-w-3xl mx-auto">
      <h1 className="text-3xl font-bold">Batch Upload Assets</h1>
      <small className="text-sm text-base-content/70">
        This Section is for large files like RAW, Video to upload. <br />
      </small>

      {isGeneratingHashCodes && hashcodeProgress && (
        <ProgressIndicator
          processed={hashcodeProgress.numberProcessed}
          total={filesCount}
          label="Generating hashes"
        />
      )}

      <button
        onClick={() => fileInputRef.current?.click()}
        className="mb-2 mt-2 btn btn-primary"
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
            setFilesCount(event.target.files.length);
          }
        }}
      />
    </section>
  );
}

export default BatchUploadSection;

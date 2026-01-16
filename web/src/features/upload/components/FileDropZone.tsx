import React from "react";
import { useUploadContext } from "@/features/upload";

type FileDropZoneProps = {
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  children?: React.ReactNode;
  onFilesDropped: (files: FileList) => void;
};

const FileDropZone = ({
  fileInputRef,
  children,
  onFilesDropped,
}: FileDropZoneProps) => {
  const { state, handleDragOver, handleDragLeave, handleDrop } =
    useUploadContext();
  const { isDragging } = state;

  // Create a wrapper for handleDrop that calls the specific file handler
  const onDrop = (e: React.DragEvent<Element>) => {
    handleDrop(e, onFilesDropped);
  };

  return (
    <div
      className={`border-2 border-dashed rounded-2xl p-16 text-center transition-all duration-200 cursor-pointer
                ${isDragging ? "border-primary bg-primary/10 scale-[0.99]" : "border-base-300 hover:border-primary/60 hover:bg-base-200/50"}`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={onDrop}
      onClick={() => fileInputRef.current?.click()}
    >
      {children}
    </div>
  );
};

export default FileDropZone;

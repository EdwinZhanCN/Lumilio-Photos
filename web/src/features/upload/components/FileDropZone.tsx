import React from "react";
import { useUploadContext } from "@/features/upload";

/**
 * Props for the FileDropZone component.
 */
type FileDropZoneProps = {
  /** Reference to the hidden file input element */
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  /** Optional child components to render inside the drop zone */
  children?: React.ReactNode;
  /** Callback function called when files are dropped */
  onFilesDropped: (files: FileList) => void;
};

/**
 * A drag-and-drop zone component for file uploads.
 * 
 * This component provides an interactive drop zone where users can:
 * - Drag and drop files directly onto the zone
 * - Click to open a file selection dialog
 * - Visual feedback during drag operations
 * 
 * The component integrates with the upload context to handle drag events
 * and provides visual feedback when files are being dragged over the zone.
 * 
 * @param props - Component props
 * @param props.fileInputRef - Reference to the hidden file input element
 * @param props.children - Optional child components to render inside the drop zone
 * @param props.onFilesDropped - Callback function called when files are dropped
 * 
 * @example
 * ```typescript
 * const fileInputRef = useRef<HTMLInputElement>(null);
 * 
 * const handleFilesDropped = (files: FileList) => {
 *   // Process dropped files
 *   uploadFiles(Array.from(files));
 * };
 * 
 * return (
 *   <FileDropZone
 *     fileInputRef={fileInputRef}
 *     onFilesDropped={handleFilesDropped}
 *   >
 *     <div>Drag files here or click to browse</div>
 *   </FileDropZone>
 * );
 * ```
 */
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

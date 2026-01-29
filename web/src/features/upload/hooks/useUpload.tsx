import { useContext } from "react";
import { UploadContext, UploadContextValue } from "../upload.type.ts";

/**
 * Custom hook for consuming the upload context.
 * 
 * Provides type-safe access to upload state and operations throughout
 * the application. This hook must be used within a component that is
 * wrapped by the UploadProvider.
 * 
 * @returns UploadContextValue containing upload state and operations
 * 
 * @throws Error if used outside of UploadProvider
 * 
 * @example
 * ```typescript
 * function MyComponent() {
 *   const { 
 *     uploadFile, 
 *     uploadProgress, 
 *     isUploading,
 *     selectedFiles 
 *   } = useUploadContext();
 *   
 *   const handleFileSelect = (files: File[]) => {
 *     uploadFile(files);
 *   };
 *   
 *   return (
 *     <div>
 *       {isUploading && <span>Uploading...</span>}
 *       <FileDropZone onFilesSelected={handleFileSelect} />
 *     </div>
 *   );
 * }
 * ```
 */
export const useUploadContext = (): UploadContextValue => {
  const context = useContext(UploadContext);
  if (context === undefined) {
    throw new Error("useUploadContext must be used within an UploadProvider");
  }
  return context;
};

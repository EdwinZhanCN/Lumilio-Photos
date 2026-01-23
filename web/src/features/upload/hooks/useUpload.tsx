import { useContext } from "react";
import { UploadContext, UploadContextValue } from "../upload.types.ts";

/**
 * Custom hook for consuming the upload context.
 * Provides type-safe access to upload state and operations.
 * @throws Error if used outside of UploadProvider
 */
export const useUploadContext = (): UploadContextValue => {
  const context = useContext(UploadContext);
  if (context === undefined) {
    throw new Error("useUploadContext must be used within an UploadProvider");
  }
  return context;
};

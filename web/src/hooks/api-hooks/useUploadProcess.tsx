import { useState, RefObject } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { formatBytes } from "@/utils/formatters";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useGenerateHashcode } from "@/hooks/wasm-hooks/useGenerateHashcode";
import { uploadService } from "@/services/uploadService";

/**
 * A single file processing result indicating a failed file with name and error.
 */
interface FailedFile {
  name: string;
  error: string;
}

/**
 * The shape of the processFiles result, containing arrays of file names that were
 * successfully uploaded, or failed.
 */
interface ProcessResults {
  uploaded: string[];
  failed: FailedFile[];
}

/**
 * The return type of the processFiles function.
 * (Accepts either FileList or File[] for flexibility)
 */
type ProcessFilesFn = (files: FileList | File[]) => Promise<ProcessResults>;

/**
 * Represents a file with its associated hash
 */
interface FileWithHash {
  file: File;
  hash: string;
}

/**
 * useUploadProcess is a custom hook that handles the upload process of files.
 * @param {React.RefObject} workerClientRef - Reference to your WASM worker client
 * @param {boolean} wasmReady - Indicates if WASM is ready
 * @returns {{
 *   processFiles: (function(*): Promise<{duplicates: *[], uploaded: *[], failed: *[]}>),
 *   isUploading: boolean,
 *   isChecking: boolean,
 *   resetStatus: Function,
 *   uploadProgress: number
 * }}
 */
export function useUploadProcess(
  workerClientRef: RefObject<any>,
  wasmReady: boolean,
): {
  processFiles: ProcessFilesFn;
  isUploading: boolean;
  resetStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: {
    numberProcessed?: number | undefined;
    total?: number | undefined;
    error?: string | undefined;
    failedAt?: number | undefined;
  } | null;
  isGeneratingHashCodes: boolean;
} {
  const queryClient = useQueryClient();

  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [hashcodeProgress, setHashcodeProgress] = useState<{
    numberProcessed?: number | undefined;
    total?: number | undefined;
    error?: string | undefined;
    failedAt?: number | undefined;
  } | null>(null);
  const [isGeneratingHashCodes, setIsGeneratingHashCodes] =
    useState<boolean>(false);
  const showMessage = useMessage();

  const handlePerformanceMetrics = (metrics: {
    numberProcessed: number;
    totalSize: number;
    processingTime: number;
    bytesPerSecond: number;
  }) => {
    const formattedSpeed = formatBytes(metrics.bytesPerSecond) + "/s";
    const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
    const formattedSize = formatBytes(metrics.totalSize);

    console.log(
      "hint",
      `Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}`,
    );
  };

  const { generateHashCodes } = useGenerateHashcode({
    setIsGeneratingHashCodes,
    setHashcodeProgress,
    onPerformanceMetrics: handlePerformanceMetrics,
    workerClientRef,
    wasmReady,
  });

  // Batch upload
  const batchUploadFiles = useMutation({
    mutationFn: (filesWithHash: FileWithHash[]) =>
      uploadService.batchUploadFiles(filesWithHash),
    onSuccess: () => {
      return queryClient.invalidateQueries({
        queryKey: ["userAssets"],
      });
    },
  });

  // File uploading
  interface UploadArgs {
    file: File;
    hash: string;
  }

  // You can further refine the mutation types here if needed
  const uploadFile = useMutation({
    mutationFn: async ({ file, hash }: UploadArgs) => {
      return await uploadService.uploadFile(file, hash, {
        onUploadProgress(progressEvent) {
          const progress = progressEvent.total
            ? Math.round((progressEvent.loaded * 100) / progressEvent.total)
            : 0;
          setUploadProgress(progress);
        },
      });
    },
    onSuccess: () => {
      return queryClient.invalidateQueries({ queryKey: ["userAssets"] });
    },
    onSettled: () => {
      // Reset progress when upload completes
      setUploadProgress(0);
    },
  });

  /**
   * Process files by generating hashes and uploading them
   */
  const processFiles: ProcessFilesFn = async (files) => {
    const results: ProcessResults = {
      uploaded: [],
      failed: [],
    };

    try {
      const startTime = performance.now();

      // Show message that we're starting to process files
      showMessage("info", "Generating file checksums...");

      // 1. Generate hashes using the WASM worker
      const hashResults = await generateHashCodes(files);

      if (hashResults instanceof Error) {
        throw hashResults;
      }

      const endTime = performance.now();
      showMessage(
        "info",
        `Checksums generated in ${((endTime - startTime) / 1000).toFixed(2)} seconds`,
      );

      // 2. Match each original file with its generated hash
      const filesWithHash: FileWithHash[] = Array.from(files).map(
        (file, index) => {
          const hashResult = hashResults?.find(
            (result: any) => result.index === index,
          );

          if (!hashResult?.hash) {
            throw new Error(`Failed to generate hash for file ${file.name}`);
          }

          return {
            file,
            hash: hashResult.hash,
          };
        },
      );

      // 3. Check if we should upload in batch or individually
      if (filesWithHash.length > 1) {
        // Batch upload
        showMessage("info", `Uploading ${filesWithHash.length} files...`);
        const response = await batchUploadFiles.mutateAsync(filesWithHash);

        // Process batch upload results
        if (response.data.results) {
          response.data.results.forEach((result: any, index: number) => {
            const fileName = filesWithHash[index]?.file.name || "unknown";
            if (result.success) {
              results.uploaded.push(fileName);
            } else {
              results.failed.push({
                name: fileName,
                error: result.message || "Upload failed",
              });
            }
          });
        }
      } else if (filesWithHash.length === 1) {
        // Single file upload
        const fileWithHash = filesWithHash[0];
        showMessage("info", `Uploading ${fileWithHash.file.name}...`);

        try {
          await uploadFile.mutateAsync({
            file: fileWithHash.file,
            hash: fileWithHash.hash,
          });

          results.uploaded.push(fileWithHash.file.name);
          showMessage(
            "success",
            `Successfully uploaded ${fileWithHash.file.name}`,
          );
        } catch (error: any) {
          results.failed.push({
            name: fileWithHash.file.name,
            error: error.message || "Upload failed",
          });
          showMessage("error", `Failed to upload ${fileWithHash.file.name}`);
        }
      }

      // Show summary
      if (results.uploaded.length > 0) {
        showMessage(
          "success",
          `Successfully uploaded ${results.uploaded.length} files`,
        );
      }

      if (results.failed.length > 0) {
        showMessage("error", `Failed to upload ${results.failed.length} files`);
      }
    } catch (error: any) {
      console.error("Upload process failed:", error);
      showMessage("error", error.message || "Upload process failed");
    }

    return results;
  };

  return {
    processFiles,
    isUploading: uploadFile.isPending || batchUploadFiles.isPending,
    resetStatus: () => {
      uploadFile.reset();
      batchUploadFiles.reset();
      setUploadProgress(0);
      setHashcodeProgress(null);
    },
    uploadProgress,
    hashcodeProgress,
    isGeneratingHashCodes,
  };
}

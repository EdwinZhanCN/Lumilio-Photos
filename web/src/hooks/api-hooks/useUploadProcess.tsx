import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { formatBytes } from "@/lib/formatters";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import {
  HashcodeProgress,
  useGenerateHashcode,
} from "@/hooks/util-hooks/useGenerateHashcode";
import { uploadService, BatchUploadResult } from "@/services/uploadService";

interface FailedFile {
  name: string;
  error: string;
}
interface ProcessResults {
  uploaded: string[];
  failed: FailedFile[];
}

type ProcessFilesFn = (files: FileList | File[]) => Promise<ProcessResults>;

interface FileWithHash {
  file: File;
  hash: string;
}

export interface useUploadProcessReturn {
  processFiles: ProcessFilesFn;
  isUploading: boolean;
  isGeneratingHashCodes: boolean;
  resetStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: HashcodeProgress | null;
}

/**
 * Custom hook for handling file upload process.
 * @returns {useUploadProcessReturn}
 */
export function useUploadProcess(): useUploadProcessReturn {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const [uploadProgress, setUploadProgress] = useState<number>(0);

  const {
    generateHashCodes,
    isGenerating: isGeneratingHashCodes,
    progress: hashcodeProgress,
  } = useGenerateHashcode((metrics) => {
    const formattedSpeed = formatBytes(metrics.bytesPerSecond) + "/s";
    const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
    const formattedSize = formatBytes(metrics.totalSize);

    console.log(
      "hash",
      `Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}!`,
    );
  });

  const uploadMutation = useMutation({
    mutationFn: (filesWithHash: FileWithHash[]) =>
      uploadService.batchUploadFiles(filesWithHash, {
        onUploadProgress(progressEvent) {
          const progress = progressEvent.total
            ? Math.round((progressEvent.loaded * 100) / progressEvent.total)
            : 0;
          setUploadProgress(progress);
        },
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["userAssets"] });
    },
    onSettled: () => {
      setUploadProgress(0);
    },
  });

  const processFiles: ProcessFilesFn = async (files) => {
    const fileArray = Array.from(files);
    if (fileArray.length === 0) {
      return { uploaded: [], failed: [] };
    }

    try {
      const hashResults = await generateHashCodes(files);
      if (!hashResults) throw new Error("Failed to generate hash codes");

      const filesWithHash: FileWithHash[] = fileArray.map((file, index) => {
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
      });

      // 统一调用批量上传
      const response = await uploadMutation.mutateAsync(filesWithHash);

      const results: BatchUploadResult[] = response.data?.data?.results || [];

      const uploaded = results.filter((r) => r.success).map((r) => r.file_name);

      const failed: FailedFile[] = results
        .filter((r) => !r.success)
        .map((r) => ({
          name: r.file_name,
          error: r.message || r.error || "Upload failed",
        }));

      const totalUploaded = uploaded.length;
      const totalFailed = failed.length;

      if (totalFailed === 0 && totalUploaded > 0) {
        // 全部成功
        showMessage(
          "success",
          `${totalUploaded} file(s) uploaded successfully.`,
        );
      } else if (totalUploaded === 0 && totalFailed > 0) {
        // 全部失败
        showMessage("error", `All ${totalFailed} file(s) failed to upload.`);
      } else if (totalUploaded > 0 && totalFailed > 0) {
        // 部分成功
        showMessage(
          "error",
          `Upload complete: ${totalUploaded} succeeded, ${totalFailed} failed.`,
        );
      }

      return { uploaded, failed };
    } catch (error: any) {
      const errorMessage =
        error.response?.data?.message ||
        error.message ||
        "An unexpected error occurred.";
      showMessage("error", errorMessage);

      return {
        uploaded: [],
        failed: fileArray.map((f) => ({
          name: f.name,
          error: "Upload process failed",
        })),
      };
    }
  };

  return {
    processFiles,
    isUploading: uploadMutation.isPending,
    isGeneratingHashCodes,
    resetStatus: () => {
      uploadMutation.reset();
      setUploadProgress(0);
    },
    uploadProgress,
    hashcodeProgress,
  };
}

import { useState, RefObject } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { formatBytes } from "@/utils/formatters";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useGenerateHashcode } from "@/hooks/wasm-hooks/useGenerateHashcode";
import { uploadService, BatchUploadResult } from "@/services/uploadService";

// (接口定义保持不变)
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

// 定义一个更统一的状态，可以按需使用
// type UploadStatus = "idle" | "hashing" | "uploading" | "error" | "success";

export function useUploadProcess(
  workerClientRef: RefObject<any>,
  wasmReady: boolean,
) {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const [uploadProgress, setUploadProgress] = useState<number>(0); // 批量上传暂时不方便计算总进度，可保留或移除
  const [hashcodeProgress, setHashcodeProgress] = useState<{
    numberProcessed?: number | undefined;
    total?: number | undefined;
    error?: string | undefined;
    failedAt?: number | undefined;
  } | null>(null);
  const [isGeneratingHashCodes, setIsGeneratingHashCodes] =
    useState<boolean>(false);

  // useGenerateHashcode 保持不变...
  const { generateHashCodes } = useGenerateHashcode({
    setIsGeneratingHashCodes,
    setHashcodeProgress,
    onPerformanceMetrics: (metrics) => {
      const formattedSpeed = formatBytes(metrics.bytesPerSecond) + "/s";
      const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
      const formattedSize = formatBytes(metrics.totalSize);

      console.log(
        "hash",
        `Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}!`,
      );
    },
    workerClientRef,
    wasmReady,
  });

  const uploadMutation = useMutation({
    // mutationFn 的返回值现在是 AxiosResponse<ApiResult<BatchUploadData>>
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
      if (hashResults instanceof Error) throw hashResults;

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

      // 2. 统一调用批量上传
      const response = await uploadMutation.mutateAsync(filesWithHash);

      // --- 关键修正：从 response.data.data.results 获取结果 ---
      const results: BatchUploadResult[] = response.data?.data?.results || [];

      const uploaded = results.filter((r) => r.success).map((r) => r.file_name);

      const failed: FailedFile[] = results
        .filter((r) => !r.success)
        .map((r) => ({
          name: r.file_name,
          error: r.message || r.error || "Upload failed",
        }));

      // --- 关键修正：统一的摘要消息逻辑 ---
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
      // 如果没有文件被处理，则不显示消息

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
      setHashcodeProgress(null);
    },
    uploadProgress,
    hashcodeProgress,
  };
}

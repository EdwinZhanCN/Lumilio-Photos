import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useWorkingRepository } from "@/features/settings";
import { useI18n } from "@/lib/i18n";
import { useGenerateHashcode } from "@/hooks/util-hooks/useGenerateHashcode";
import type { HashcodeProgress } from "@/hooks/util-hooks/useGenerateHashcode";
import { useBatchUploadMutation, useChunkedUploadMutation } from "./useUploadMutations.ts";
import { useUploadConfig } from "./useUploadQueries.ts";
import { resolveUploadTransportConfig } from "./uploadProcessConfig.ts";
import { useUploadProgressState } from "./uploadProcessProgress.ts";
import { isDuplicateResult, summarizeUploadResults } from "./uploadProcessResults.ts";
import { runUploadProcess } from "./uploadProcessRunner.ts";
import type { FileUploadProgress, ProcessFilesFn, ProcessResults } from "./uploadProcessTypes.ts";

export type { FileUploadProgress } from "./uploadProcessTypes.ts";

export interface useUploadProcessReturn {
  processFiles: ProcessFilesFn;
  isUploading: boolean;
  isGeneratingHashCodes: boolean;
  resetStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: HashcodeProgress | null;
  fileProgress: FileUploadProgress[];
}

/**
 * React adapter for the upload process. The transport and hashing pipeline live
 * in separate modules so this hook only owns UI state, app services, and notices.
 */
export function useUploadProcess(): useUploadProcessReturn {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { t } = useI18n();
  const { scopedRepositoryId } = useWorkingRepository();
  const [isUploading, setIsUploading] = useState(false);
  const {
    uploadProgress,
    fileProgress,
    setUploadProgress,
    initializeFileProgress,
    updateFileProgress,
    reset: resetProgress,
  } = useUploadProgressState();
  const { mutateAsync: batchUpload } = useBatchUploadMutation();
  const { mutateAsync: chunkedUpload } = useChunkedUploadMutation();
  const uploadConfigQuery = useUploadConfig();
  const {
    generateHashCodes,
    isGenerating: isGeneratingHashCodes,
    progress: hashcodeProgress,
  } = useGenerateHashcode();

  const invalidateAssetQueries = useCallback(() => {
    return queryClient.resetQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (!Array.isArray(key)) return false;
        const path = key[1];
        return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
      },
    });
  }, [queryClient]);

  const processFiles = useCallback<ProcessFilesFn>(
    async (files) => {
      const fileArray = Array.from(files);
      if (fileArray.length === 0) return { uploaded: [], duplicates: [], failed: [] };

      setIsUploading(true);
      const messages = {
        noResult: t("upload.UploadProcess.noResult"),
        processFailed: t("upload.UploadProcess.processFailed"),
        uploadFailed: t("upload.UploadProcess.uploadFailed"),
      };

      try {
        const runResult = await runUploadProcess(files, {
          repositoryId: scopedRepositoryId,
          config: resolveUploadTransportConfig(uploadConfigQuery.data),
          messages,
          generateHashCodes,
          initializeFileProgress,
          updateFileProgress,
          setUploadProgress,
          batchUpload,
          chunkedUpload,
        });

        if (runResult.results.some((result) => result.success && !isDuplicateResult(result))) {
          await invalidateAssetQueries();
        }

        const summary = summarizeUploadResults(
          runResult.results,
          runResult.resultSessions,
          fileArray[0],
          messages.uploadFailed,
        );
        showUploadSummary(summary, t, showMessage);
        return summary;
      } catch (error: unknown) {
        const message = error instanceof Error ? error.message : messages.processFailed;
        showMessage("error", message);
        return {
          uploaded: [],
          duplicates: [],
          failed: fileArray.map((file) => ({
            name: file.name,
            error: messages.processFailed,
            file,
          })),
        };
      } finally {
        setIsUploading(false);
        setUploadProgress(0);
      }
    },
    [
      batchUpload,
      chunkedUpload,
      generateHashCodes,
      initializeFileProgress,
      invalidateAssetQueries,
      scopedRepositoryId,
      setUploadProgress,
      showMessage,
      t,
      updateFileProgress,
      uploadConfigQuery.data,
    ],
  );

  return {
    processFiles,
    isUploading,
    isGeneratingHashCodes,
    resetStatus: resetProgress,
    uploadProgress,
    hashcodeProgress,
    fileProgress,
  };
}

function showUploadSummary(
  summary: ProcessResults,
  t: ReturnType<typeof useI18n>["t"],
  showMessage: ReturnType<typeof useMessage>,
): void {
  const { uploaded, duplicates, failed } = summary;
  if (failed.length > 0) {
    showMessage(
      "error",
      t("upload.UploadProcess.summaryPartial", {
        succeeded: uploaded.length,
        failed: failed.length,
      }),
    );
  } else if (duplicates.length > 0) {
    showMessage(
      uploaded.length > 0 ? "success" : "info",
      t(
        "upload.UploadProcess.summaryDuplicates",
        "Uploaded {{count}} files, skipped {{duplicates}} already in your library.",
        {
          count: uploaded.length,
          duplicates: duplicates.length,
        },
      ),
    );
  } else if (uploaded.length > 0) {
    showMessage("success", t("upload.UploadProcess.summarySuccess", { count: uploaded.length }));
  }
}

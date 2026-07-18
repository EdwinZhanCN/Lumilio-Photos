import { useCallback, useEffect, useMemo, useState } from "react";
import {
  getRetryTasksByCategoryForAssetType,
  isRetryTaskSupportedForAssetType,
} from "@/config/retryTasks";
import { useMessage } from "@/features/notifications";
import type { Asset } from "@/lib/assets/types";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";

export function useAssetReprocess(asset?: Asset) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [selectedTasks, setSelectedTasks] = useState<string[]>([]);
  const [forceFullRetry, setForceFullRetry] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const mutation = $api.useMutation("post", "/api/v1/assets/{id}/reprocess");
  const tasksByCategory = useMemo(
    () => getRetryTasksByCategoryForAssetType(asset?.type),
    [asset?.type],
  );
  const supportedSelectedTasks = useMemo(
    () => selectedTasks.filter((task) => isRetryTaskSupportedForAssetType(task, asset?.type)),
    [asset?.type, selectedTasks],
  );
  const hasRetryableTasks = Object.values(tasksByCategory).some((tasks) => tasks.length > 0);
  const canSubmit =
    Boolean(asset?.asset_id) &&
    !isRetrying &&
    (forceFullRetry || supportedSelectedTasks.length > 0);

  useEffect(() => {
    setSelectedTasks((current) =>
      current.filter((task) => isRetryTaskSupportedForAssetType(task, asset?.type)),
    );
  }, [asset?.type]);

  const toggleTask = useCallback((task: string, selected: boolean) => {
    setSelectedTasks((current) =>
      selected ? Array.from(new Set([...current, task])) : current.filter((item) => item !== task),
    );
  }, []);

  const submit = useCallback(async () => {
    if (!asset?.asset_id || (!forceFullRetry && supportedSelectedTasks.length === 0)) return;
    setIsRetrying(true);
    try {
      await mutation.mutateAsync({
        params: { path: { id: asset.asset_id } },
        body: {
          tasks: forceFullRetry ? [] : supportedSelectedTasks,
          force_full_retry: forceFullRetry,
        },
      });
      document.querySelector<HTMLDialogElement>("#asset_retry_dialog")?.close();
      setSelectedTasks([]);
      setForceFullRetry(false);
      showMessage(
        "success",
        t("exportModal.retrySubmitted", { defaultValue: "Processing retry submitted." }),
      );
    } catch (error) {
      console.error("Failed to submit retry job:", error);
      showMessage(
        "error",
        t("exportModal.retrySubmitFailed", {
          defaultValue: "Failed to submit the processing retry.",
        }),
      );
    } finally {
      setIsRetrying(false);
    }
  }, [asset?.asset_id, forceFullRetry, mutation, showMessage, supportedSelectedTasks, t]);

  return {
    tasksByCategory,
    selectedTasks,
    supportedSelectedCount: supportedSelectedTasks.length,
    forceFullRetry,
    isRetrying,
    hasRetryableTasks,
    canSubmit,
    toggleTask,
    setForceFullRetry,
    submit,
  };
}

export type AssetReprocessController = ReturnType<typeof useAssetReprocess>;

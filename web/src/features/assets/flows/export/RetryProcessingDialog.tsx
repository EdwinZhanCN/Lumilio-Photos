import { Loader2, RefreshCw, X } from "lucide-react";
import type { RetryTaskOption } from "@/config/retryTasks";
import { useI18n } from "@/lib/i18n";
import type { AssetReprocessController } from "./useAssetReprocess";

type RetryTaskCategoryProps = {
  label: string;
  tasks: RetryTaskOption[];
  controller: AssetReprocessController;
};

function RetryTaskCategory({ label, tasks, controller }: RetryTaskCategoryProps) {
  const { t } = useI18n();
  if (tasks.length === 0) return null;

  return (
    <section className="overflow-hidden rounded-lg border border-base-300">
      <h4 className="bg-base-200/50 px-3 py-1.5 text-xs font-semibold uppercase tracking-wider opacity-70">
        {label}
      </h4>
      <div className="divide-y divide-base-200">
        {tasks.map((task) => (
          <label
            key={task.key}
            className="flex cursor-pointer items-start gap-3 px-3 py-2.5 transition-colors hover:bg-base-200/50"
          >
            <input
              type="checkbox"
              className="checkbox checkbox-sm mt-0.5"
              checked={controller.selectedTasks.includes(task.key)}
              onChange={(event) => controller.toggleTask(task.key, event.target.checked)}
              disabled={controller.isRetrying}
            />
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium">
                {t(`exportModal.retryTasks.${task.key}.label`, { defaultValue: task.label })}
              </span>
              <span className="mt-0.5 block text-xs opacity-60">
                {t(`exportModal.retryTasks.${task.key}.description`, {
                  defaultValue: task.description,
                })}
              </span>
            </span>
          </label>
        ))}
      </div>
    </section>
  );
}

export function RetryProcessingDialog({ controller }: { controller: AssetReprocessController }) {
  const { t } = useI18n();
  return (
    <dialog id="asset_retry_dialog" className="modal">
      <div className="modal-box max-w-lg">
        <form method="dialog">
          <button
            className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2"
            aria-label={t("common.close")}
          >
            <X />
          </button>
        </form>
        <h3 className="mb-1 text-lg font-semibold">
          {t("exportModal.retryTitle", { defaultValue: "Retry Processing Tasks" })}
        </h3>
        <p className="mb-4 text-xs opacity-60">
          {t("exportModal.selectedCount", {
            defaultValue: "{{count}} selected",
            count: controller.supportedSelectedCount,
          })}
        </p>

        <div className="flex max-h-[60vh] flex-col gap-3 overflow-y-auto pr-1">
          <RetryTaskCategory
            label={t("exportModal.category.metadata", { defaultValue: "Metadata" })}
            tasks={controller.tasksByCategory.metadata}
            controller={controller}
          />
          <RetryTaskCategory
            label={t("exportModal.category.media", { defaultValue: "Media Processing" })}
            tasks={controller.tasksByCategory.media}
            controller={controller}
          />
          <RetryTaskCategory
            label={t("exportModal.category.ml", { defaultValue: "Machine Learning" })}
            tasks={controller.tasksByCategory.ml}
            controller={controller}
          />
          {!controller.hasRetryableTasks && (
            <p className="py-4 text-center text-sm opacity-70">
              {t("exportModal.noRetryTasks", {
                defaultValue: "No retry tasks are available for this asset type.",
              })}
            </p>
          )}
          <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-base-300 p-3">
            <input
              type="checkbox"
              className="checkbox checkbox-sm mt-0.5"
              checked={controller.forceFullRetry}
              onChange={(event) => controller.setForceFullRetry(event.target.checked)}
              disabled={controller.isRetrying}
            />
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium">
                {t("exportModal.forceFullRetry", { defaultValue: "Force full retry" })}
              </span>
              <span className="mt-0.5 block text-xs opacity-60">
                {t("exportModal.forceFullRetryHint", {
                  defaultValue: "Re-run all processing tasks regardless of previous status",
                })}
              </span>
            </span>
          </label>
        </div>

        <div className="modal-action mt-4 border-t border-base-300 pt-3">
          <form method="dialog">
            <button className="btn btn-ghost btn-sm" disabled={controller.isRetrying}>
              {t("common.cancel")}
            </button>
          </form>
          <button
            className="btn btn-primary btn-sm"
            disabled={!controller.canSubmit}
            onClick={() => void controller.submit()}
          >
            {controller.isRetrying ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                {t("exportModal.submitting", { defaultValue: "Submitting..." })}
              </>
            ) : (
              <>
                <RefreshCw className="size-4" />
                {t("exportModal.submitRetry", { defaultValue: "Submit Retry" })}
              </>
            )}
          </button>
        </div>
      </div>
    </dialog>
  );
}

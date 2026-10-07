import { useMessage } from "@/features/notifications";
import { RotateCcw } from "lucide-react";
import { localizeAPIProblem, normalizeProblem } from "@/lib/http-commons/problem";
import { useAssetLifecycle } from "@/lib/assets/useAssetLifecycle";
import { useI18n } from "@/lib/i18n";

/** Presentation of the typed availability Problem; catalog metadata stays readable. */
export function AssetAvailabilityNotice({
  error,
  assetId,
  onRetry,
}: {
  error: unknown;
  assetId: string;
  onRetry?: () => void;
}) {
  const { t } = useI18n();
  const { restore } = useAssetLifecycle();
  const showMessage = useMessage();
  const handleRestore = async () => {
    try {
      const result = await restore.mutateAsync({ body: { asset_ids: [assetId] } });
      if (result.renamed?.length)
        showMessage(
          "info",
          t(
            "assets.lifecycle.restoredNames",
            "Occupied paths were preserved. Restored files: {{paths}}",
            { paths: result.renamed.map((file) => file.restored_path).join(", ") },
          ),
        );
    } catch {
      /* The structured failure is rendered below. */
    }
  };
  const problem = normalizeProblem(error);
  const trashed =
    problem.kind === "problem" && problem.type === "https://lumilio.org/problems/asset/trashed";
  return (
    <div
      role="status"
      className="flex h-full min-h-64 flex-col items-center justify-center gap-4 p-8 text-center"
    >
      <p>
        {localizeAPIProblem(
          error,
          t,
          t("assets.lifecycle.unavailable", "The original is unavailable."),
        )}
      </p>
      {trashed && (
        <button
          className="btn btn-primary"
          disabled={restore.isPending}
          onClick={() => void handleRestore()}
        >
          <RotateCcw size={16} />
          {t("assets.lifecycle.restore", "Restore")}
        </button>
      )}
      {onRetry && (
        <button className="btn btn-ghost" onClick={onRetry}>
          {t("common.retry", "Retry")}
        </button>
      )}
      {restore.error && (
        <p role="alert">
          {localizeAPIProblem(
            restore.error,
            t,
            t("assets.lifecycle.actionFailed", "The lifecycle action could not be completed."),
          )}
        </p>
      )}
    </div>
  );
}

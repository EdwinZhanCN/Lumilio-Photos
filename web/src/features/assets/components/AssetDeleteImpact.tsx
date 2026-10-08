import { useEffect } from "react";
import { useAssetDeleteImpact } from "../api/useAssetDeleteImpact";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";

/** Catalog preview only; the delete command rechecks every file before moving. */
export function AssetDeleteImpact({
  assetIds,
  onReady,
}: {
  assetIds: string[];
  onReady: (ready: boolean) => void;
}) {
  const { t } = useI18n();
  const query = useAssetDeleteImpact(assetIds);
  useEffect(() => {
    onReady(query.isSuccess && Boolean(query.data));
  }, [query.data, query.isSuccess, onReady]);
  if (query.isError)
    return (
      <div role="alert">
        {localizeAPIProblem(
          query.error,
          t,
          t("assets.lifecycle.previewFailed", "The deletion preview could not be loaded."),
        )}
        <button className="btn btn-sm" onClick={() => void query.refetch()}>
          {t("common.retry", "Retry")}
        </button>
      </div>
    );
  if (!query.data) return <span className="loading loading-spinner" />;
  const impact = query.data;
  return (
    <div className="space-y-2 text-sm">
      <p>
        {t("assets.lifecycle.impact", "{{assets}} assets · {{files}} files · {{bytes}}", {
          assets: impact.assets,
          files: impact.files,
          bytes: formatBytes(impact.bytes ?? 0),
        })}
      </p>
      <p>
        {t("assets.lifecycle.repositories", "Repositories: {{names}}", {
          names: impact.repositories?.map((repo) => repo.name).join(", "),
        })}
      </p>
      <p>
        {t(
          "assets.lifecycle.retention",
          "Files move to the Trash and can be restored for {{days}} days. After that they are permanently deleted.",
          { days: impact.retention_days },
        )}
      </p>
    </div>
  );
}

import { AlertTriangle } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useLifecycleAudit } from "../../api/useStorageDiagnostics";
import { formatDateTime } from "./storageTime";

/**
 * The durable administrative storage audit trail, shown as a plain list. It is
 * the History view of the storage page, so it owns no disclosure of its own and
 * no row expansion: one row per recorded action.
 */
export default function LifecycleHistory() {
  const { t, i18n } = useI18n();
  const audit = useLifecycleAudit(true);
  const events = audit.data?.events ?? [];

  if (audit.isLoading) {
    return (
      <div className="space-y-2 p-4" aria-hidden>
        <div className="skeleton h-3 w-36" />
        <div className="skeleton h-3 w-2/3" />
        <div className="skeleton h-3 w-1/2" />
      </div>
    );
  }

  if (audit.isError) {
    return (
      <div role="alert" className="flex items-center gap-2 px-4 py-5 text-sm text-error">
        <AlertTriangle size={16} aria-hidden />
        <span>{t("storagePanel.audit.loadFailed", "Lifecycle history could not be loaded.")}</span>
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <p className="m-0 px-4 py-6 text-center text-sm text-base-content/50">
        {t("storagePanel.audit.empty", "No lifecycle actions have been recorded yet.")}
      </p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="table table-sm">
        <thead>
          <tr>
            <th scope="col">{t("storagePanel.audit.time", "Time")}</th>
            <th scope="col">{t("storagePanel.audit.action", "Action")}</th>
            <th scope="col">{t("storagePanel.audit.target", "Target")}</th>
            <th scope="col">{t("storagePanel.audit.actor", "Actor")}</th>
            <th scope="col">{t("storagePanel.audit.result", "Result")}</th>
          </tr>
        </thead>
        <tbody>
          {events.map((event) => (
            <tr key={event.event_id}>
              <td className="whitespace-nowrap text-xs">
                {formatDateTime(event.occurred_at, i18n.language, t("common.na"))}
              </td>
              <td className="font-mono text-xs">{event.action || t("common.na")}</td>
              <td className="text-xs">
                <span>{event.target_type || t("common.na")}</span>
                {event.target_id ? (
                  <span
                    className="mt-0.5 block max-w-36 truncate font-mono text-[11px] text-base-content/45"
                    title={event.target_id}
                  >
                    {event.target_id}
                  </span>
                ) : null}
              </td>
              <td className="text-xs">{event.actor || t("common.na")}</td>
              <td>
                <ResultBadge result={event.result} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ResultBadge({ result }: { result?: string }) {
  const { t } = useI18n();
  const succeeded = result === "succeeded" || result === "recovered";
  const resultClass = !result
    ? "badge-ghost"
    : succeeded
      ? "badge-success"
      : result === "rejected"
        ? "badge-warning"
        : "badge-error";
  return (
    <span className={`badge badge-sm badge-soft ${resultClass}`}>{result || t("common.na")}</span>
  );
}

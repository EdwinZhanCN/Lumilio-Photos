import { AlertTriangle, ChevronDown, History } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useLifecycleAudit } from "@/features/repositories";

/**
 * LifecycleHistory renders the durable administrative storage audit trail as a
 * collapsible table below the master-detail storage view. It owns its own
 * query so the fixed-height master-detail pane never competes for vertical
 * space with the history list.
 */
export function LifecycleHistory() {
  const { t } = useI18n();
  const audit = useLifecycleAudit(true);

  return (
    <details className="group rounded-lg border border-base-300 bg-base-100 shadow-sm">
      <summary className="flex cursor-pointer select-none items-center gap-3 px-4 py-3 sm:px-5">
        <span className="flex size-8 shrink-0 items-center justify-center rounded-field bg-base-200 text-base-content/65">
          <History size={15} />
        </span>
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">
            {t("monitor.storage.auditTitle", "Lifecycle history")}
          </h2>
          <p className="truncate text-xs text-base-content/50">
            {t("monitor.storage.auditDescription", "Durable administrative storage actions")}
          </p>
        </div>
        <ChevronDown className="ml-auto size-4 shrink-0 text-base-content/45 transition-transform group-open:rotate-180" />
      </summary>
      <div className="border-t border-base-300">
        {audit.isLoading ? (
          <div className="space-y-2 p-4" aria-hidden="true">
            <div className="skeleton h-3 w-36" />
            <div className="skeleton h-3 w-2/3" />
            <div className="skeleton h-3 w-1/2" />
          </div>
        ) : audit.isError ? (
          <div role="alert" className="flex items-center gap-2 px-4 py-5 text-sm text-error">
            <AlertTriangle size={16} />
            <span>
              {t("monitor.storage.auditLoadFailed", "Lifecycle history could not be loaded.")}
            </span>
          </div>
        ) : (audit.data?.events ?? []).length === 0 ? (
          <p className="px-4 py-5 text-sm text-base-content/50">
            {t("monitor.storage.auditEmpty", "No lifecycle actions have been recorded yet.")}
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="table table-sm min-w-[40rem]">
              <thead className="bg-base-200/25 text-[11px] text-base-content/60">
                <tr>
                  <th>{t("monitor.storage.auditTime", "Time")}</th>
                  <th>{t("monitor.storage.auditAction", "Action")}</th>
                  <th>{t("monitor.storage.auditTarget", "Target")}</th>
                  <th>{t("monitor.storage.auditActor", "Actor")}</th>
                  <th>{t("monitor.storage.auditResult", "Result")}</th>
                </tr>
              </thead>
              <tbody>
                {(audit.data?.events ?? []).map((event) => (
                  <tr key={event.event_id} className="hover:bg-base-200/30">
                    <td className="whitespace-nowrap text-xs">
                      {formatTime(event.occurred_at, t("common.na"))}
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
        )}
      </div>
    </details>
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

function formatTime(value: string | undefined, emptyValue: string): string {
  if (!value) return emptyValue;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

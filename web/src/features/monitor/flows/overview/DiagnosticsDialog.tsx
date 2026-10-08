import { X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useProcessingDiagnostics } from "../../api/useProcessing";
import { QueueSummaryList } from "./QueueSummaryList";

/** River delivery records: execution attempts, never file progress. */
export function DiagnosticsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useI18n();
  const query = useProcessingDiagnostics(open);
  if (!open) return null;
  const data = query.data;
  const totals = [
    [
      t("monitor.delivery.active", "Active deliveries"),
      (data?.available ?? 0) +
        (data?.scheduled ?? 0) +
        (data?.running ?? 0) +
        (data?.retryable ?? 0),
    ],
    [t("monitor.delivery.completed", "Completed deliveries"), data?.completed ?? 0],
    [t("monitor.delivery.discarded", "Discarded deliveries"), data?.discarded ?? 0],
    [t("monitor.delivery.cancelled", "Cancelled deliveries"), data?.cancelled ?? 0],
  ] as const;
  return (
    <div
      className="modal modal-open z-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="processing-diagnostics"
    >
      <div className="modal-box flex max-w-3xl flex-col gap-4">
        <div className="flex items-center gap-2">
          <h2 id="processing-diagnostics" className="flex-1 text-lg font-semibold">
            {t("monitor.processing.diagnostics.title", "Diagnostics")}
          </h2>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-square"
            onClick={onClose}
            aria-label={t("common.close", "Close")}
          >
            <X className="size-4" aria-hidden />
          </button>
        </div>
        {query.isError ? (
          <div role="alert" className="alert alert-warning text-sm">
            {t("monitor.stats.fetchError")}
          </div>
        ) : !data ? (
          <div className="py-8 text-center text-sm text-base-content/60">{t("common.loading")}</div>
        ) : (
          <>
            <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">
              {totals.map(([label, value]) => (
                <div key={label}>
                  <dt className="text-sm text-base-content/60">{label}</dt>
                  <dd className="mt-1 text-lg font-semibold tabular-nums">{value}</dd>
                </div>
              ))}
            </dl>
            <QueueSummaryList queues={data.queues ?? []} />
          </>
        )}
      </div>
      <button
        type="button"
        className="modal-backdrop"
        aria-label={t("common.close", "Close")}
        onClick={onClose}
      />
    </div>
  );
}

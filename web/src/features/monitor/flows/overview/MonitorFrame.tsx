import type { ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { formatDiagnosticTime } from "../../model/time";
import "./monitor.css";

/**
 * Shared snapshot chrome; freshness describes reads, never a repair action.
 *
 * The heading is deliberately one toolbar row: the tab already names itself in
 * the route header, so a repeated title, count, and refresh-mode sentence would
 * only restate it. Every tab renders the same row, and the row carries the one
 * fact no other surface owns — when the shown snapshot was read.
 */
export function MonitorFrame({
  children,
  actions,
  isLoading,
  hasData,
  error,
  isFetching,
  onRefresh,
  updatedAt,
}: {
  children?: ReactNode;
  actions?: ReactNode;
  isLoading?: boolean;
  hasData: boolean;
  error?: string;
  isFetching?: boolean;
  onRefresh: () => void;
  updatedAt?: number | string;
}) {
  const { t } = useI18n();
  const timestamp =
    typeof updatedAt === "number"
      ? updatedAt
        ? new Date(updatedAt).toISOString()
        : undefined
      : updatedAt;
  return (
    <section className="monitor-frame space-y-5">
      <header className="flex min-h-8 flex-wrap items-center gap-3">
        {timestamp && (
          <p className="text-xs text-base-content/60">
            {t("monitor.snapshot.updated", "Last success {{time}}", {
              time: formatDiagnosticTime(timestamp, t("common.na")),
            })}
          </p>
        )}
        <div className="ml-auto flex flex-wrap items-center gap-2">
          {actions}
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onRefresh}
            disabled={isFetching}
          >
            <RefreshCw
              className={`size-4 ${isFetching ? "motion-safe:animate-spin" : ""}`}
              aria-hidden
            />
            {t("settings.serverSettings.refresh")}
          </button>
        </div>
      </header>
      {error && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm"
        >
          <AlertTriangle className="size-4 shrink-0" aria-hidden />
          <div>
            {error}
            {hasData && (
              <p className="mt-1">
                {t(
                  "monitor.snapshot.stale",
                  "Refresh failed. Showing the last successful snapshot; current state is unknown.",
                )}
              </p>
            )}
          </div>
        </div>
      )}
      {!hasData ? (
        <div
          className="flex min-h-80 items-center justify-center text-sm text-base-content/60"
          aria-busy={isLoading}
        >
          {isLoading
            ? t("common.loading")
            : !error
              ? t("monitor.snapshot.missing", "No snapshot is available.")
              : null}
        </div>
      ) : (
        children
      )}
    </section>
  );
}

export function MonitorDiagnostics({ children, title }: { children: ReactNode; title?: string }) {
  const { t } = useI18n();
  return (
    <details className="monitor-diagnostics border-t border-base-300 pt-4">
      <summary className="cursor-pointer text-sm font-medium">
        {title ?? t("monitor.snapshot.diagnostics", "Diagnostics")}
      </summary>
      <div className="mt-4 space-y-4">{children}</div>
    </details>
  );
}

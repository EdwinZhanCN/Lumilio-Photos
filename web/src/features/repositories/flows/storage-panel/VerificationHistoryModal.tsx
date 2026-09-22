import { useEffect, useMemo, useState } from "react";
import { History } from "lucide-react";
import Modal from "@/components/ui/Modal";
import type { components } from "@/lib/http-commons/schema";
import { useI18n } from "@/lib/i18n";
import {
  useRepositoryVerificationDetail,
  useRepositoryVerificationList,
} from "../../api/useRepositoryVerifications";
import { formatDateTime } from "./storageTime";

type RepositoryScanRunDTO = components["schemas"]["dto.RepositoryScanRunDTO"];

function Fact({ term, value }: { term: string; value: string }) {
  return (
    <div className="flex min-w-0 items-baseline gap-3">
      <dt className="w-28 shrink-0 text-xs text-base-content/55">{term}</dt>
      <dd className="m-0 min-w-0 break-words text-xs text-base-content/85">{value}</dd>
    </div>
  );
}

function RunDetail({ run }: { run: RepositoryScanRunDTO }) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;

  return (
    <dl className="m-0 flex flex-col gap-1.5">
      <Fact
        term={t("storagePanel.verificationHistory.fieldStatus", "Status")}
        value={run.status ?? "—"}
      />
      <Fact
        term={t("storagePanel.verificationHistory.fieldMode", "Mode")}
        value={run.mode ?? "—"}
      />
      <Fact
        term={t("storagePanel.verificationHistory.fieldStarted", "Started")}
        value={formatDateTime(run.started_at, locale)}
      />
      <Fact
        term={t("storagePanel.verificationHistory.fieldFinished", "Finished")}
        value={formatDateTime(run.finished_at, locale)}
      />
      <Fact
        term={t("storagePanel.verificationHistory.fieldPartialCoverage", "Partial coverage")}
        value={run.partial_coverage ? t("common.yes", "Yes") : t("common.no", "No")}
      />
      <Fact
        term={t(
          "storagePanel.verificationHistory.fieldErrorDirectories",
          "Directories with errors",
        )}
        value={String(run.error_directories ?? 0)}
      />
      {run.operation_id ? (
        <Fact
          term={t("storagePanel.verificationHistory.fieldOperation", "Operation")}
          value={run.operation_id}
        />
      ) : null}
    </dl>
  );
}

/**
 * Scan history as a master-detail view: the runs are a scrollable list on the
 * left, the selected run's facts fill the right pane. Neither pane is the modal
 * body — the body does not scroll, so the list keeps its own scroll position
 * while the detail changes.
 */
export default function VerificationHistoryModal({
  repositoryId,
  repositoryName,
  isOpen,
  onClose,
}: {
  repositoryId: string;
  repositoryName: string;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t, i18n } = useI18n();
  const listQuery = useRepositoryVerificationList(repositoryId, isOpen);
  const runs = useMemo(() => listQuery.data?.scans ?? [], [listQuery.data?.scans]);
  const [selectedOperationId, setSelectedOperationId] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (!isOpen) {
      setSelectedOperationId(undefined);
      return;
    }
    if (runs.length === 0) {
      setSelectedOperationId(undefined);
      return;
    }
    setSelectedOperationId((current) => {
      if (current && runs.some((run) => run.operation_id === current)) {
        return current;
      }
      return runs[0]?.operation_id;
    });
  }, [isOpen, runs]);

  const detailQuery = useRepositoryVerificationDetail(
    repositoryId,
    selectedOperationId,
    isOpen && Boolean(selectedOperationId),
  );

  const selectedRun =
    detailQuery.data ?? runs.find((run) => run.operation_id === selectedOperationId);

  return (
    <Modal
      open={isOpen}
      onClose={onClose}
      title={t("storagePanel.verificationHistory.title", "Scan history")}
      icon={<History size={20} aria-hidden />}
      size="lg"
      bodyScrollable={false}
      bodyClassName="flex flex-col sm:flex-row"
    >
      <div className="flex max-h-56 min-h-0 shrink-0 flex-col border-b border-base-200 sm:max-h-none sm:w-72 sm:border-b-0 sm:border-r">
        <p className="px-3 pb-1 pt-3 text-xs font-medium text-base-content/55">{repositoryName}</p>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {listQuery.isLoading ? (
            <div className="flex flex-col gap-2 p-1" aria-hidden>
              <div className="skeleton h-10 w-full" />
              <div className="skeleton h-10 w-full" />
              <div className="skeleton h-10 w-full" />
            </div>
          ) : listQuery.isError ? (
            <p role="alert" className="px-1 text-xs text-error">
              {t(
                "storagePanel.verificationHistory.loadFailed",
                "Scan history could not be loaded.",
              )}
            </p>
          ) : runs.length === 0 ? (
            <p className="px-1 text-xs text-base-content/55">
              {t("storagePanel.verificationHistory.empty", "No scans yet.")}
            </p>
          ) : (
            <ul className="menu menu-sm w-full p-0">
              {runs.map((run) => {
                const operationId = run.operation_id ?? "";
                const selected = operationId === selectedOperationId;
                return (
                  <li key={operationId || run.created_at}>
                    <button
                      type="button"
                      className={selected ? "menu-active" : undefined}
                      aria-current={selected ? "true" : undefined}
                      onClick={() => setSelectedOperationId(operationId)}
                    >
                      <span className="flex min-w-0 flex-1 flex-col items-start gap-0.5">
                        <span className="truncate capitalize">{run.status ?? "—"}</span>
                        {/* opacity, not a colour: the selected item must keep
                            the theme's menu-active foreground. */}
                        <span className="text-[11px] tabular-nums opacity-60">
                          {formatDateTime(run.started_at ?? run.created_at, i18n.language)}
                        </span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        {selectedRun ? (
          <RunDetail run={selectedRun} />
        ) : detailQuery.isLoading ? (
          <div className="flex flex-col gap-2" aria-hidden>
            <div className="skeleton h-4 w-40" />
            <div className="skeleton h-4 w-56" />
            <div className="skeleton h-4 w-48" />
          </div>
        ) : (
          <p className="m-0 text-xs text-base-content/55">
            {t("storagePanel.verificationHistory.selectRun", "Select a scan to see its details.")}
          </p>
        )}
      </div>
    </Modal>
  );
}

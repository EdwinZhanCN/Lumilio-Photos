import { useEffect, useMemo, useState } from "react";
import { CircleAlert, CopyCheck, Loader2, Upload } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useUploadContext } from "../../state/useUploadContext";
import type { FileUploadProgress } from "../../modules/process/useUploadProcess";
import { shouldUseChunks } from "@/lib/upload/uploadTransport";

const RECENTLY_FINISHED_VISIBILITY_MS = 90_000;

type UploadRowStatus = FileUploadProgress["status"];

function getStatusBadgeClass(status: UploadRowStatus) {
  switch (status) {
    case "completed":
      return "badge-success";
    case "uploading":
      return "badge-primary";
    case "duplicate":
      return "badge-warning";
    case "failed":
      return "badge-error";
    case "pending":
    default:
      return "badge-ghost";
  }
}

function getProgressClass(status: UploadRowStatus) {
  switch (status) {
    case "completed":
      return "progress-success";
    case "uploading":
      return "progress-primary";
    case "duplicate":
      return "progress-warning";
    case "failed":
      return "progress-error";
    case "pending":
    default:
      return "progress-base-300";
  }
}

function getStatusOrder(status: UploadRowStatus) {
  switch (status) {
    case "uploading":
    case "processing":
      return 0;
    case "pending":
      return 1;
    case "failed":
      return 2;
    case "duplicate":
      return 3;
    case "completed":
    default:
      return 4;
  }
}

type TranslateFn = ReturnType<typeof useI18n>["t"];

function getStatusLabel(t: TranslateFn, status: UploadRowStatus) {
  switch (status) {
    case "pending":
      return t("upload.FileUploadProgress.status_pending");
    case "uploading":
      return t("upload.FileUploadProgress.status_uploading");
    case "processing":
      return t("upload.FileUploadProgress.status_processing", "Processing");
    case "completed":
      return t("upload.FileUploadProgress.status_completed");
    case "duplicate":
      return t("upload.FileUploadProgress.status_duplicate", "Duplicate");
    case "failed":
      return t("upload.FileUploadProgress.status_failed");
  }
}

export default function NavbarUploadQueue() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { state, fileProgress, isProcessing } = useUploadContext();
  const [isVisible, setIsVisible] = useState(false);

  const pendingSelectionItems = useMemo<FileUploadProgress[]>(
    () =>
      state.files.map((file, index) => ({
        fileName: file.name,
        progress: 0,
        status: "pending",
        sessionId: `selected-${index}-${file.name}`,
        isChunked: shouldUseChunks(file),
        error: undefined,
      })),
    [state.files],
  );

  const sourceItems = fileProgress.length > 0 ? fileProgress : pendingSelectionItems;

  const orderedItems = useMemo(
    () =>
      [...sourceItems].sort((a, b) => {
        const statusOrder = getStatusOrder(a.status) - getStatusOrder(b.status);
        if (statusOrder !== 0) return statusOrder;
        return a.fileName.localeCompare(b.fileName);
      }),
    [sourceItems],
  );

  const activeCount = orderedItems.filter(
    (item) =>
      item.status === "pending" || item.status === "uploading" || item.status === "processing",
  ).length;
  const failedCount = orderedItems.filter((item) => item.status === "failed").length;
  // Duplicates are a success path — they must not put the queue into an error state.
  const duplicateCount = orderedItems.filter((item) => item.status === "duplicate").length;

  useEffect(() => {
    if (orderedItems.length === 0) {
      setIsVisible(false);
      return;
    }

    if (state.files.length > 0 || activeCount > 0 || isProcessing || failedCount > 0) {
      setIsVisible(true);
      return;
    }

    setIsVisible(true);
    const timeoutId = window.setTimeout(() => {
      setIsVisible(false);
    }, RECENTLY_FINISHED_VISIBILITY_MS);

    return () => window.clearTimeout(timeoutId);
  }, [activeCount, failedCount, isProcessing, orderedItems.length, state.files.length]);

  if (!isVisible || orderedItems.length === 0) {
    return (
      <button
        type="button"
        className="btn btn-sm sm:btn-md btn-ghost gap-1 sm:gap-2 rounded-full px-2 sm:px-3"
        onClick={() => navigate("/manage")}
        aria-label={t("upload.NavbarQueue.openPage")}
        title={t("upload.NavbarQueue.openPage")}
      >
        <Upload className="w-5 h-5" />
      </button>
    );
  }

  const leadingIcon =
    activeCount > 0 ? (
      <Loader2 className="size-4 animate-spin text-primary" />
    ) : failedCount > 0 ? (
      <CircleAlert className="size-4 text-error" />
    ) : duplicateCount > 0 ? (
      <CopyCheck className="size-4 text-warning" />
    ) : (
      <Upload className="size-4 text-primary" />
    );

  return (
    <div className="dropdown dropdown-end">
      <button
        type="button"
        tabIndex={0}
        className="btn btn-sm sm:btn-md btn-ghost gap-1 sm:gap-2 rounded-full px-2 sm:px-3"
      >
        {leadingIcon}
        <span className="badge badge-primary badge-sm">{orderedItems.length}</span>
        {duplicateCount > 0 && (
          <span className="badge badge-warning badge-sm">{duplicateCount}</span>
        )}
        {failedCount > 0 && <span className="badge badge-error badge-sm">{failedCount}</span>}
      </button>

      <div className="dropdown-content z-30 mt-2 w-96 max-w-[calc(100vw-2rem)] rounded-2xl border border-base-300 bg-base-100 shadow-xl">
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <div>
            <h3 className="font-semibold">{t("upload.NavbarQueue.title")}</h3>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-base-content/60">
              <span>{t("upload.NavbarQueue.active", { count: activeCount })}</span>
              {duplicateCount > 0 && (
                <span className="text-warning">
                  {t("upload.NavbarQueue.duplicate", "{{count}} duplicate", {
                    count: duplicateCount,
                  })}
                </span>
              )}
              {failedCount > 0 && (
                <span>{t("upload.NavbarQueue.failed", { count: failedCount })}</span>
              )}
            </div>
          </div>
        </div>

        <ul className="max-h-96 overflow-y-auto divide-y divide-base-300/50">
          {orderedItems.map((item) => (
            <li key={item.sessionId} className="px-4 py-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <p className="truncate text-sm font-medium" title={item.fileName}>
                      {item.fileName}
                    </p>
                    {item.isChunked && (
                      <span className="badge badge-xs badge-outline">
                        {t("upload.FileUploadProgress.chunked_badge")}
                      </span>
                    )}
                  </div>
                  {item.error && (
                    <p className="mt-1 truncate text-xs text-error" title={item.error}>
                      {item.error}
                    </p>
                  )}
                </div>
                <span className={`badge badge-sm ${getStatusBadgeClass(item.status)}`}>
                  {getStatusLabel(t, item.status)}
                </span>
              </div>

              <div className="mt-2 flex items-center gap-2">
                <progress
                  className={`progress h-2 w-full ${getProgressClass(item.status)}`}
                  value={item.progress}
                  max="100"
                />
                <span className="min-w-10 text-right text-xs font-medium tabular-nums text-base-content/70">
                  {Math.round(item.progress)}%
                </span>
              </div>
            </li>
          ))}
        </ul>

        <div className="space-y-3 border-t border-base-300 px-4 py-3">
          <p className="text-xs text-base-content/60">{t("upload.NavbarQueue.backgroundHint")}</p>
          <Link to="/manage" className="btn btn-primary btn-sm w-full rounded-full">
            {t("upload.NavbarQueue.openPage")}
          </Link>
        </div>
      </div>
    </div>
  );
}

import { useMemo, useState } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { AlertTriangle, Check, Copy, Loader2, Trash2, X } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import BrowseScopeSelect from "@/components/BrowseScopeSelect";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useBrowseScope } from "@/features/settings";
import { assetUrls } from "@/lib/assets/assetUrls";
import { formatBytes } from "@/lib/utils/formatters";
import type { DuplicateGroup, DuplicateMethod, DuplicateStatus } from "@/lib/duplicates/types";
import {
  useDismissDuplicateGroup,
  useDuplicateGroupList,
  useDuplicateSummary,
  useMergeDuplicateGroup,
} from "../hooks/useDuplicates";

const STATUS_OPTIONS: DuplicateStatus[] = ["pending", "merged", "dismissed"];

type StatusBadgeProps = {
  method?: string;
};

function getMethodLabel(t: (key: string) => string, method: DuplicateMethod) {
  switch (method) {
    case "exact":
      return t("duplicates.method.exact");
    case "mixed":
      return t("duplicates.method.mixed");
    case "phash":
      return t("duplicates.method.phash");
  }
}

function getStatusFilterLabel(t: (key: string) => string, status: DuplicateStatus) {
  switch (status) {
    case "pending":
      return t("duplicates.filters.pending");
    case "merged":
      return t("duplicates.filters.merged");
    case "dismissed":
      return t("duplicates.filters.dismissed");
  }
}

const StatusBadge = ({ method }: StatusBadgeProps) => {
  const { t } = useI18n();
  const m = (method ?? "phash") as DuplicateMethod;
  const color = m === "exact" ? "badge-error" : m === "mixed" ? "badge-warning" : "badge-info";
  return <span className={`badge badge-sm ${color} badge-outline`}>{getMethodLabel(t, m)}</span>;
};

type DuplicateGroupCardProps = {
  group: DuplicateGroup;
  status: DuplicateStatus;
};

const DuplicateGroupCard = ({ group, status }: DuplicateGroupCardProps) => {
  const { t } = useI18n();
  const showMessage = useMessage();
  const mergeMutation = useMergeDuplicateGroup();
  const dismissMutation = useDismissDuplicateGroup();

  const initialKeeperId =
    group.keeper_asset_id ??
    group.recommended_keeper_asset_id ??
    group.assets?.[0]?.asset?.asset_id ??
    null;
  const [selectedKeeperId, setSelectedKeeperId] = useState<string | null>(initialKeeperId);

  const isResolved = status !== "pending";
  const isDismissed = status === "dismissed";
  const assets = group.assets ?? [];
  const groupId = group.group_id ?? "";
  const isActionPending = mergeMutation.isPending || dismissMutation.isPending;

  const recoverableBytes = group.recoverable_bytes ?? 0;
  const totalBytes = group.total_size ?? 0;

  const handleMerge = async () => {
    if (!groupId || !selectedKeeperId) return;
    try {
      const result = await mergeMutation.mutateAsync({
        groupId,
        body: {
          keeper_asset_id: selectedKeeperId,
        },
      });
      showMessage(
        "success",
        t("duplicates.group.mergeSuccess", {
          size: formatBytes(result.recovered_bytes ?? 0),
        }),
      );
    } catch (err) {
      showMessage(
        "error",
        t("duplicates.group.mergeError", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  const handleDismiss = async () => {
    if (!groupId) return;
    try {
      await dismissMutation.mutateAsync({ groupId });
      showMessage("success", t("duplicates.group.dismissSuccess"));
    } catch (err) {
      showMessage(
        "error",
        t("duplicates.group.dismissError", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  return (
    <article className="rounded-[1.75rem] border border-base-300/60 bg-base-100 p-5 shadow-sm">
      <header className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge method={group.method} />
          <span className="text-sm font-semibold">
            {t("duplicates.group.membersCount", { count: assets.length })}
          </span>
          <span className="text-xs text-base-content/55">
            {t("duplicates.group.totalSize", {
              size: formatBytes(totalBytes),
            })}
          </span>
          {recoverableBytes > 0 && (
            <span className="badge badge-sm badge-ghost">
              {t("duplicates.group.saveSize", {
                size: formatBytes(recoverableBytes),
              })}
            </span>
          )}
        </div>
        {!isResolved && (
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-ghost btn-sm rounded-full"
              onClick={handleDismiss}
              disabled={isActionPending}
            >
              {dismissMutation.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  {t("duplicates.group.dismissing")}
                </>
              ) : (
                <>
                  <X className="size-4" />
                  {t("duplicates.group.dismiss")}
                </>
              )}
            </button>
            <button
              type="button"
              className="btn btn-primary btn-sm rounded-full"
              onClick={handleMerge}
              disabled={!selectedKeeperId || isActionPending || assets.length < 2}
            >
              {mergeMutation.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  {t("duplicates.group.merging")}
                </>
              ) : (
                <>
                  <Check className="size-4" />
                  {t("duplicates.group.merge")}
                </>
              )}
            </button>
          </div>
        )}
      </header>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {assets.map((member) => {
          const assetId = member.asset?.asset_id;
          const isKeeper = assetId === selectedKeeperId;
          const wasResolvedKeeper = isResolved && !isDismissed && assetId === group.keeper_asset_id;
          const wasResolvedDuplicate = isResolved && !isDismissed && !wasResolvedKeeper;
          return (
            <button
              key={assetId}
              type="button"
              onClick={() => !isResolved && assetId && setSelectedKeeperId(assetId)}
              disabled={isResolved}
              className={[
                "group relative overflow-hidden rounded-2xl border bg-base-200 text-left transition",
                isResolved ? "cursor-default" : "cursor-pointer",
                isKeeper ? "border-primary ring-2 ring-primary/60" : "border-transparent",
                wasResolvedDuplicate ? "opacity-60" : "",
              ].join(" ")}
            >
              <div className="relative aspect-square w-full">
                {assetId ? (
                  <img
                    src={assetUrls.getThumbnailUrl(assetId, "medium")}
                    alt={member.asset?.original_filename ?? assetId}
                    loading="lazy"
                    className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.02]"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center bg-base-300">
                    <Copy className="size-8 text-base-content/30" />
                  </div>
                )}
                {isKeeper && !isResolved && (
                  <span className="absolute left-2 top-2 badge badge-primary badge-sm">
                    <Check className="size-3" />
                    {t("duplicates.group.keeperLabel")}
                  </span>
                )}
                {!isKeeper && !isResolved && (
                  <span className="absolute left-2 top-2 badge badge-ghost badge-sm">
                    {t("duplicates.group.candidateLabel")}
                  </span>
                )}
                {wasResolvedKeeper && (
                  <span className="absolute left-2 top-2 badge badge-success badge-sm">
                    <Check className="size-3" />
                    {t("duplicates.group.keeperLabel")}
                  </span>
                )}
                {wasResolvedDuplicate && (
                  <span className="absolute left-2 top-2 badge badge-error badge-sm">
                    <Trash2 className="size-3" />
                    {t("duplicates.group.duplicateLabel")}
                  </span>
                )}
                {isDismissed && (
                  <span className="absolute left-2 top-2 badge badge-ghost badge-sm">
                    {t("duplicates.group.dismissedLabel")}
                  </span>
                )}
              </div>
              <div className="space-y-0.5 px-3 py-2">
                <p className="truncate text-xs font-medium">
                  {member.asset?.original_filename ?? assetId}
                </p>
                <p className="text-[11px] text-base-content/55">
                  {formatBytes(member.file_size ?? 0)}
                </p>
              </div>
            </button>
          );
        })}
      </div>
    </article>
  );
};

function DuplicatesContent() {
  const { t } = useI18n();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    {
      label: t("collections.sections.utilities", "Utilities"),
      to: "/collections/utilities",
    },
    { label: t("duplicates.pageTitle", "Duplicates") },
  ]);
  const { scopedRepositoryId } = useBrowseScope();
  const [status, setStatus] = useState<DuplicateStatus>("pending");
  const summaryQuery = useDuplicateSummary(scopedRepositoryId);
  const groupQuery = useDuplicateGroupList({
    repositoryId: scopedRepositoryId,
    status,
    limit: 50,
  });

  const summary = summaryQuery.data;

  const isInitialLoading = groupQuery.isLoading;
  const hasGroups = groupQuery.groups.length > 0;
  const hasSummaryError = summaryQuery.isError;

  const lastDetectedLabel = useMemo(() => {
    if (!summary?.last_detected_at) {
      return t("duplicates.summary.lastDetected") + ": " + t("duplicates.summary.neverDetected");
    }
    return (
      t("duplicates.summary.lastDetected") +
      ": " +
      new Date(summary.last_detected_at).toLocaleString()
    );
  }, [summary?.last_detected_at, t]);

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("duplicates.pageTitle")}
        icon={<Copy className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
      </PageHeader>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <SummaryStat
              label={t("duplicates.summary.pendingGroups")}
              value={(summary?.pending_groups ?? 0).toString()}
            />
            <SummaryStat
              label={t("duplicates.summary.recoverableAssets")}
              value={(summary?.recoverable_assets ?? 0).toString()}
            />
            <SummaryStat
              label={t("duplicates.summary.recoverableBytes")}
              value={formatBytes(summary?.recoverable_bytes ?? 0)}
            />
            <SummaryStat
              label={t("duplicates.summary.lastDetected")}
              value={
                summary?.last_detected_at
                  ? new Date(summary.last_detected_at).toLocaleDateString()
                  : "—"
              }
              title={lastDetectedLabel}
            />
          </section>

          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wide text-base-content/55">
              {t("duplicates.filters.status")}
            </span>
            <div className="join">
              {STATUS_OPTIONS.map((opt) => (
                <button
                  key={opt}
                  type="button"
                  onClick={() => setStatus(opt)}
                  className={`btn btn-sm join-item ${status === opt ? "btn-primary" : "btn-ghost"}`}
                >
                  {getStatusFilterLabel(t, opt)}
                </button>
              ))}
            </div>
          </div>

          {hasSummaryError && (
            <div className="alert alert-warning">
              <AlertTriangle className="size-5" />
              <span>
                {t("duplicates.summaryLoadError", {
                  message:
                    summaryQuery.error instanceof Error
                      ? summaryQuery.error.message
                      : String(summaryQuery.error ?? ""),
                })}
              </span>
            </div>
          )}

          {groupQuery.isError && (
            <div className="alert alert-warning">
              <AlertTriangle className="size-5" />
              <span>
                {t("duplicates.loadError", {
                  message:
                    groupQuery.error instanceof Error
                      ? groupQuery.error.message
                      : (JSON.stringify(groupQuery.error ?? "") ?? ""),
                })}
              </span>
            </div>
          )}

          {isInitialLoading && (
            <div className="space-y-3">
              {Array.from({ length: 2 }).map((_, idx) => (
                <div key={idx} className="h-48 animate-pulse rounded-[1.75rem] bg-base-200" />
              ))}
            </div>
          )}

          {!isInitialLoading && !hasGroups && (
            <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-12 text-center text-base-content/60">
              <p className="text-base font-semibold">{t("duplicates.emptyTitle")}</p>
              <p className="mt-1 text-sm">
                {status === "pending" && summary && summary.last_detected_at == null
                  ? t("duplicates.noScanYet")
                  : t("duplicates.emptyDescription")}
              </p>
            </div>
          )}

          {hasGroups && (
            <div className="space-y-4">
              {groupQuery.groups.map((group) => (
                <DuplicateGroupCard key={group.group_id} group={group} status={status} />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

type SummaryStatProps = {
  label: string;
  value: string;
  title?: string;
};

const SummaryStat = ({ label, value, title }: SummaryStatProps) => (
  <div className="rounded-2xl bg-base-200/60 px-4 py-3" title={title}>
    <p className="text-xs font-semibold uppercase tracking-wide text-base-content/55">{label}</p>
    <p className="mt-1 truncate text-lg font-black">{value}</p>
  </div>
);

export default function Duplicates() {
  const { t } = useI18n();
  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <DuplicatesContent />
    </ErrorBoundary>
  );
}

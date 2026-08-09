import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  BookImage,
  ChevronDown,
  ChevronRight,
  Download,
  Folder,
  FolderOpen,
  HardDrive,
  RefreshCw,
  X,
} from "lucide-react";
import type { components } from "@/lib/http-commons/schema";
import { useI18n } from "@/lib/i18n";
import { useStorageDiagnostics, useStorageSupportBundle } from "@/features/repositories";
import { StorageStatusDot, StorageTargetDetail, storageItemSeverity } from "./StorageTargetDetail";

type StorageDiagnostic = components["schemas"]["dto.StorageDiagnosticDTO"];

export function StorageMonitor() {
  const { t } = useI18n();
  const diagnostics = useStorageDiagnostics(true);

  const hierarchy = useMemo(() => {
    const items = diagnostics.data?.items ?? [];
    const locations = items.filter((item) => item.target_type === "storage_location");
    const repositoriesByLocation = new Map<string, StorageDiagnostic[]>();
    const unlinkedRepositories: StorageDiagnostic[] = [];
    const locationIDs = new Set(locations.map((location) => location.target_id));

    for (const repository of items.filter((item) => item.target_type === "repository")) {
      const parentID = repository.parent_target_id;
      if (!parentID || !locationIDs.has(parentID)) {
        unlinkedRepositories.push(repository);
        continue;
      }
      const repositories = repositoriesByLocation.get(parentID) ?? [];
      repositories.push(repository);
      repositoriesByLocation.set(parentID, repositories);
    }

    const repositoryCount =
      unlinkedRepositories.length +
      locations.reduce(
        (count, location) =>
          count + (repositoriesByLocation.get(location.target_id ?? "")?.length ?? 0),
        0,
      );
    const attentionCount = items.filter((item) => storageItemSeverity(item) !== "healthy").length;

    return {
      locations,
      repositoriesByLocation,
      unlinkedRepositories,
      repositoryCount,
      attentionCount,
    };
  }, [diagnostics.data?.items]);

  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [mobileTreeOpen, setMobileTreeOpen] = useState(false);
  const selected = useMemo(() => {
    const candidates = [
      ...hierarchy.locations,
      ...hierarchy.locations.flatMap(
        (location) => hierarchy.repositoriesByLocation.get(location.target_id ?? "") ?? [],
      ),
      ...hierarchy.unlinkedRepositories,
    ];
    return (
      candidates.find((item) => item.target_id === selectedID) ??
      hierarchy.locations[0] ??
      hierarchy.unlinkedRepositories[0]
    );
  }, [selectedID, hierarchy]);

  useEffect(() => {
    if (!mobileTreeOpen) return;

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setMobileTreeOpen(false);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [mobileTreeOpen]);

  const selectMobileTarget = (targetID: string) => {
    setSelectedID(targetID);
    setMobileTreeOpen(false);
  };

  return (
    <div className="flex h-auto w-full min-w-0 max-w-full flex-col gap-4">
      {diagnostics.isLoading ? (
        <StorageSkeleton />
      ) : diagnostics.isError ? (
        <div className="flex min-h-0 flex-1 flex-col gap-4">
          <StorageToolbar isFetching={diagnostics.isFetching} onRefresh={diagnostics.refetch} />
          <div role="alert" className="alert alert-error alert-soft">
            <AlertTriangle size={18} />
            <span>
              {t("monitor.storage.loadFailed", "Storage diagnostics could not be loaded.")}
            </span>
            <button type="button" className="btn btn-sm" onClick={() => void diagnostics.refetch()}>
              {t("common.retry", "Retry")}
            </button>
          </div>
        </div>
      ) : hierarchy.locations.length === 0 && hierarchy.unlinkedRepositories.length === 0 ? (
        <div className="flex min-h-0 flex-1 flex-col gap-4">
          <StorageToolbar isFetching={diagnostics.isFetching} onRefresh={diagnostics.refetch} />
          <div className="rounded-lg bg-base-100 p-6 text-center shadow-sm">
            <HardDrive className="mx-auto size-8 text-base-content/30" />
            <h3 className="mt-3 text-sm font-semibold">
              {t("monitor.storage.emptyTitle", "No storage diagnostics")}
            </h3>
            <p className="mt-1 text-sm text-base-content/55">
              {t(
                "monitor.storage.emptyDescription",
                "No registered storage targets are available.",
              )}
            </p>
          </div>
        </div>
      ) : (
        <>
          <StorageToolbar
            isFetching={diagnostics.isFetching}
            onRefresh={diagnostics.refetch}
            generatedAt={diagnostics.data?.generated_at}
            locationCount={hierarchy.locations.length}
            repositoryCount={hierarchy.repositoryCount}
            attentionCount={hierarchy.attentionCount}
          />

          {/* 主从区：左树独立滚动，右侧详情按内容自然展开 */}
          <div className="relative grid h-auto w-full min-w-0 max-w-full grid-cols-1 items-stretch gap-4 lg:grid-cols-[20rem_minmax(0,1fr)]">
            {/* 移动端：当前目标选择器 */}
            <div className="lg:hidden">
              <button
                type="button"
                className="flex min-h-12 w-full min-w-0 items-center gap-3 rounded-lg border border-base-300 bg-base-100 px-3 py-2 text-left shadow-sm transition-colors hover:bg-base-200/60 active:bg-base-200"
                aria-expanded={mobileTreeOpen}
                aria-haspopup="dialog"
                aria-controls="mobile-storage-targets"
                onClick={() => setMobileTreeOpen(true)}
              >
                {selected?.target_type === "repository" ? (
                  <BookImage className="size-4 shrink-0 text-base-content/55" aria-hidden />
                ) : (
                  <Folder className="size-4 shrink-0 text-base-content/55" aria-hidden />
                )}
                <span className="min-w-0 flex-1">
                  <span className="block text-[10px] font-semibold tracking-wider text-base-content/45 uppercase">
                    {t("monitor.storage.navLabel", "Storage targets")}
                  </span>
                  <span className="block truncate text-sm font-medium">
                    {selected?.name || t("common.na")}
                  </span>
                </span>
                <ChevronDown className="size-4 shrink-0 text-base-content/45" aria-hidden />
              </button>
            </div>

            {/* 移动端：Bottom Sheet 文件树 */}
            {mobileTreeOpen ? (
              <div className="fixed inset-0 z-50 lg:hidden">
                <button
                  type="button"
                  className="absolute inset-0 bg-base-content/20 backdrop-blur-[1px]"
                  aria-label={t("common.close", "Close")}
                  onClick={() => setMobileTreeOpen(false)}
                />
                <section
                  id="mobile-storage-targets"
                  role="dialog"
                  aria-modal="true"
                  aria-labelledby="mobile-storage-targets-title"
                  className="absolute inset-x-0 bottom-0 flex max-h-[70dvh] min-h-0 flex-col rounded-t-xl bg-base-100 shadow-2xl"
                >
                  <div
                    className="mx-auto mt-2 h-1 w-10 shrink-0 rounded-full bg-base-content/15"
                    aria-hidden="true"
                  />
                  <header className="flex shrink-0 items-center justify-between border-b border-base-content/10 px-4 py-3">
                    <div className="min-w-0">
                      <h2 id="mobile-storage-targets-title" className="text-sm font-semibold">
                        {t("monitor.storage.navLabel", "Storage targets")}
                      </h2>
                      <p className="mt-0.5 truncate text-xs text-base-content/50">
                        {selected?.name || t("common.na")}
                      </p>
                    </div>
                    <button
                      type="button"
                      className="btn btn-square btn-ghost btn-sm shrink-0"
                      onClick={() => setMobileTreeOpen(false)}
                      aria-label={t("common.close", "Close")}
                    >
                      <X className="size-4" aria-hidden />
                    </button>
                  </header>
                  <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 pt-2 pb-[calc(1rem+env(safe-area-inset-bottom))]">
                    <TargetNav
                      hierarchy={hierarchy}
                      selectedID={selected?.target_id}
                      onSelect={selectMobileTarget}
                    />
                  </div>
                </section>
              </div>
            ) : null}
            {/* 桌面端：左侧文件树，固定宽度，垂直滚动 */}
            <aside className="hidden h-full min-h-0 overflow-y-auto lg:block">
              <TargetNav
                hierarchy={hierarchy}
                selectedID={selected?.target_id}
                onSelect={setSelectedID}
              />
            </aside>
            {/* 右侧信息面板：保持卡片，内容由外层页面自然展开 */}
            <section className="w-full min-w-0 max-w-full">
              {selected ? (
                <StorageTargetDetail
                  item={selected}
                  repositories={
                    selected.target_type === "storage_location"
                      ? (hierarchy.repositoriesByLocation.get(selected.target_id ?? "") ?? [])
                      : []
                  }
                />
              ) : null}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

function StorageToolbar({
  isFetching,
  onRefresh,
  generatedAt,
  locationCount,
  repositoryCount,
  attentionCount,
}: {
  isFetching: boolean;
  onRefresh: () => void;
  generatedAt?: string;
  locationCount?: number;
  repositoryCount?: number;
  attentionCount?: number;
}) {
  const { t } = useI18n();
  const supportBundle = useStorageSupportBundle();
  const hasData = locationCount !== undefined;

  const downloadSupportBundle = async () => {
    const result = await supportBundle.refetch();
    if (!result.data) return;
    const blob = new Blob([JSON.stringify(result.data, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "lumilio-storage-support.json";
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-base-300 bg-base-100 px-4 py-3 shadow-sm">
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        {hasData ? (
          attentionCount === 0 ? (
            <span className="inline-flex items-center gap-1.5 text-sm font-medium text-success">
              <span className="size-2 rounded-full bg-success" aria-hidden="true" />
              {t("monitor.storage.allHealthy", "All storage targets are healthy")}
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 text-sm font-medium text-warning">
              <AlertTriangle className="size-4" />
              {t("monitor.storage.attentionNeeded", "{{count}} targets need attention", {
                count: attentionCount,
              })}
            </span>
          )
        ) : null}
        <span className="text-xs text-base-content/50 tabular-nums">
          {hasData
            ? t(
                "monitor.storage.summaryMeta",
                "{{locations}} locations · {{repositories}} repositories",
                {
                  locations: locationCount,
                  repositories: repositoryCount,
                },
              )
            : null}
          {generatedAt
            ? ` · ${t("monitor.storage.updated", "Checked {{time}}", {
                time: formatTime(generatedAt, t("common.na")),
              })}`
            : null}
        </span>
      </div>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className="btn btn-ghost btn-sm gap-2"
          disabled={isFetching}
          onClick={() => onRefresh()}
        >
          {isFetching ? (
            <span className="loading loading-spinner loading-xs" />
          ) : (
            <RefreshCw size={15} />
          )}
          {t("monitor.storage.refresh", "Refresh")}
        </button>
        <button
          type="button"
          className="btn btn-ghost btn-sm gap-2"
          disabled={supportBundle.isFetching}
          onClick={() => void downloadSupportBundle()}
        >
          {supportBundle.isFetching ? (
            <span className="loading loading-spinner loading-xs" />
          ) : (
            <Download size={15} />
          )}
          {t("monitor.storage.download", "Download support bundle")}
        </button>
      </div>
    </div>
  );
}

function TargetNav({
  hierarchy,
  selectedID,
  onSelect,
}: {
  hierarchy: {
    locations: StorageDiagnostic[];
    repositoriesByLocation: Map<string, StorageDiagnostic[]>;
    unlinkedRepositories: StorageDiagnostic[];
  };
  selectedID?: string;
  onSelect: (targetID: string) => void;
}) {
  const { t } = useI18n();
  const [openLocations, setOpenLocations] = useState<Set<string>>(
    () => new Set(hierarchy.locations.map((location) => location.target_id ?? "")),
  );

  const toggleLocation = (targetID: string) => {
    setOpenLocations((previous) => {
      const next = new Set(previous);
      if (next.has(targetID)) {
        next.delete(targetID);
      } else {
        next.add(targetID);
      }
      return next;
    });
  };

  return (
    <nav className="w-full min-w-0" aria-label={t("monitor.storage.navLabel", "Storage targets")}>
      <ul className="m-0 w-full min-w-0 space-y-1 p-0 lg:space-y-0.5">
        {hierarchy.locations.map((location) => {
          const targetID = location.target_id ?? "";
          const repositories = hierarchy.repositoriesByLocation.get(targetID) ?? [];
          const isOpen = openLocations.has(targetID);
          const isSelected = targetID === selectedID;
          return (
            <li key={targetID}>
              {/* 整行一个按钮：点击 chevron 图标展开/收起，点击其他区域只选中 */}
              <button
                type="button"
                onClick={(event) => {
                  if ((event.target as Element).closest("[data-chevron]")) {
                    toggleLocation(targetID);
                  } else {
                    onSelect(targetID);
                  }
                }}
                className={`flex min-h-11 w-full min-w-0 cursor-pointer items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm lg:min-h-0 lg:gap-1.5 lg:rounded-field lg:px-1.5 lg:py-1 ${
                  isSelected
                    ? "bg-primary/10 text-primary lg:bg-neutral lg:text-neutral-content"
                    : "hover:bg-base-content/10"
                }`}
                aria-expanded={isOpen}
                aria-label={location.name}
                aria-current={isSelected ? "true" : undefined}
              >
                {isOpen ? (
                  <ChevronDown
                    data-chevron
                    className={`size-4 shrink-0 ${isSelected ? "text-primary/60 lg:text-neutral-content/60" : "text-base-content/40"}`}
                    strokeWidth={1.5}
                    aria-hidden
                  />
                ) : (
                  <ChevronRight
                    data-chevron
                    className={`size-4 shrink-0 ${isSelected ? "text-primary/60 lg:text-neutral-content/60" : "text-base-content/40"}`}
                    strokeWidth={1.5}
                    aria-hidden
                  />
                )}
                {isOpen ? (
                  <FolderOpen className="size-4 shrink-0" strokeWidth={1.5} aria-hidden />
                ) : (
                  <Folder className="size-4 shrink-0" strokeWidth={1.5} aria-hidden />
                )}
                <span className="min-w-0 truncate">{location.name}</span>
                <StorageStatusDot item={location} className="ml-auto" />
              </button>
              {isOpen && repositories.length > 0 ? (
                <ul className="ml-4 border-l border-base-content/10 pl-2 lg:ml-3.5">
                  {repositories.map((repository) => {
                    const isRepoSelected = repository.target_id === selectedID;
                    return (
                      <li key={repository.target_id}>
                        <button
                          type="button"
                          onClick={() => onSelect(repository.target_id ?? "")}
                          className={`flex min-h-11 w-full min-w-0 cursor-pointer items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm lg:min-h-0 lg:gap-1.5 lg:rounded-field lg:px-1.5 lg:py-1 ${
                            isRepoSelected
                              ? "bg-primary/10 text-primary lg:bg-neutral lg:text-neutral-content"
                              : "hover:bg-base-content/10"
                          }`}
                          aria-current={isRepoSelected ? "true" : undefined}
                        >
                          <BookImage className="size-4 shrink-0" strokeWidth={1.5} aria-hidden />
                          <span className="min-w-0 truncate">{repository.name}</span>
                          <StorageStatusDot item={repository} className="ml-auto" />
                        </button>
                      </li>
                    );
                  })}
                </ul>
              ) : null}
            </li>
          );
        })}
        {hierarchy.unlinkedRepositories.length > 0 ? (
          <>
            <li className="mt-1 border-t border-base-content/10 pt-1">
              <span className="pointer-events-none flex select-none items-center gap-1.5 px-2 py-1 text-xs font-medium text-warning">
                <AlertTriangle className="size-3.5 shrink-0" aria-hidden />
                {t("monitor.storage.unlinkedTitle", "Repositories without a known location")}
              </span>
            </li>
            {hierarchy.unlinkedRepositories.map((repository) => {
              const isRepoSelected = repository.target_id === selectedID;
              return (
                <li key={repository.target_id}>
                  <button
                    type="button"
                    onClick={() => onSelect(repository.target_id ?? "")}
                    className={`flex min-h-11 w-full min-w-0 cursor-pointer items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm lg:min-h-0 lg:gap-1.5 lg:rounded-field lg:px-1.5 lg:py-1 ${
                      isRepoSelected
                        ? "bg-primary/10 text-primary lg:bg-neutral lg:text-neutral-content"
                        : "hover:bg-base-content/10"
                    }`}
                    aria-current={isRepoSelected ? "true" : undefined}
                  >
                    <BookImage className="size-4 shrink-0" strokeWidth={1.5} aria-hidden />
                    <span className="min-w-0 truncate">{repository.name}</span>
                    <StorageStatusDot item={repository} className="ml-auto" />
                  </button>
                </li>
              );
            })}
          </>
        ) : null}
      </ul>
    </nav>
  );
}

function StorageSkeleton() {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4" aria-hidden="true">
      <div className="flex items-center justify-between rounded-lg border border-base-300 bg-base-100 px-4 py-3 shadow-sm">
        <div className="space-y-1.5">
          <div className="skeleton h-3.5 w-44" />
          <div className="skeleton h-3 w-32" />
        </div>
        <div className="flex gap-2">
          <div className="skeleton h-7 w-20 rounded-field" />
          <div className="skeleton h-7 w-36 rounded-field" />
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row">
        <div className="hidden w-80 shrink-0 space-y-2 rounded-box bg-base-200 p-3 shadow-sm lg:block">
          {[0, 1, 2].map((item) => (
            <div key={item} className="skeleton h-7 w-full rounded-field" />
          ))}
        </div>
        <div className="min-h-0 flex-1 space-y-3 rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
          <div className="skeleton h-5 w-40" />
          <div className="skeleton h-3 w-2/3" />
          <div className="skeleton mt-4 h-2 w-full max-w-md" />
          <div className="skeleton mt-4 h-16 w-full" />
        </div>
      </div>
    </div>
  );
}

function formatTime(value: string | undefined, emptyValue: string): string {
  if (!value) return emptyValue;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

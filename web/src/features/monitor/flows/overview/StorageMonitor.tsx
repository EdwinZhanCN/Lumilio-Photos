import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  BookImage,
  ChevronDown,
  ChevronRight,
  CircleAlert,
  Download,
  Folder,
  FolderOpen,
  HardDrive,
  X,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import {
  getStorageEntityDisplayName,
  type StorageDiagnostic,
  useStorageDiagnostics,
  useStorageSupportBundle,
} from "@/features/repositories";
import { StorageStatusDot, StorageTargetDetail } from "./StorageTargetDetail";
import { storageItemSeverity } from "../../model/storageSeverity";
import { MonitorFrame } from "./MonitorFrame";

export function StorageMonitor() {
  const { t } = useI18n();
  const diagnostics = useStorageDiagnostics(true);

  const hierarchy = useMemo(() => {
    const items = diagnostics.data?.items ?? [];
    const locations = items.filter((item) => item.entityType === "storage_location");
    const repositoriesByLocation = new Map<string, StorageDiagnostic[]>();
    const unlinkedRepositories: StorageDiagnostic[] = [];
    const locationIDs = new Set(locations.map((location) => location.target_id));

    for (const repository of items.filter((item) => item.entityType === "repository")) {
      const parentID = repository.parent_target_id;
      if (!parentID || !locationIDs.has(parentID)) {
        unlinkedRepositories.push(repository);
        continue;
      }
      const repositories = repositoriesByLocation.get(parentID) ?? [];
      repositories.push(repository);
      repositoriesByLocation.set(parentID, repositories);
    }

    // The tree is the enumeration of every target, so the tab no longer derives
    // a second "N Storage Locations · M Repositories" tally for the heading.
    const attentionCount = items.filter((item) => storageItemSeverity(item) !== "healthy").length;

    return {
      locations,
      repositoriesByLocation,
      unlinkedRepositories,
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
    <MonitorFrame
      hasData={!!diagnostics.data}
      isLoading={diagnostics.isLoading}
      isFetching={diagnostics.isFetching}
      onRefresh={() => void diagnostics.refetch()}
      updatedAt={diagnostics.data?.generated_at}
      error={
        diagnostics.isError
          ? t("monitor.storage.loadFailed", "Storage diagnostics could not be loaded.")
          : undefined
      }
      actions={<SupportBundleButton />}
    >
      {hierarchy.locations.length === 0 && hierarchy.unlinkedRepositories.length === 0 ? (
        <div className="min-h-80 py-16 text-center">
          <HardDrive className="mx-auto size-8 text-base-content/30" />
          <h3 className="mt-3 text-sm font-semibold">
            {t("monitor.storage.emptyTitle", "No storage diagnostics")}
          </h3>
          <p className="mt-1 text-sm text-base-content/55">
            {t("monitor.storage.emptyDescription", "No registered storage targets are available.")}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {/* 关注计数属于整个 Tab 而非某一列，所以提到主从区之上；健康时不作声，
              陈旧快照也不得断言当前状态。 */}
          {!diagnostics.isError && hierarchy.attentionCount > 0 && (
            <p className="flex items-center gap-1.5 text-sm text-warning">
              <CircleAlert className="size-4 shrink-0" aria-hidden />
              {t("monitor.storage.attentionNeeded", "{{count}} targets need attention", {
                count: hierarchy.attentionCount,
              })}
            </p>
          )}
          {/* 主从区：左树独立滚动，右侧详情按内容自然展开 */}
          <div className="relative grid h-auto w-full min-w-0 max-w-full grid-cols-1 items-stretch gap-4 lg:grid-cols-[15rem_minmax(0,1fr)]">
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
                {selected?.entityType === "repository" ? (
                  <BookImage className="size-4 shrink-0 text-base-content/55" aria-hidden />
                ) : (
                  <Folder className="size-4 shrink-0 text-base-content/55" aria-hidden />
                )}
                <span className="min-w-0 flex-1">
                  <span className="block text-[10px] font-semibold tracking-wider text-base-content/45 uppercase">
                    {t("monitor.storage.navLabel", "Storage targets")}
                  </span>
                  <span className="block truncate text-sm font-medium">
                    {selected ? getStorageEntityDisplayName(selected, t) : t("common.na")}
                  </span>
                </span>
                <ChevronDown className="size-4 shrink-0 text-base-content/45" aria-hidden />
              </button>
            </div>

            {/* 移动端：Bottom Sheet 文件树 */}
            {mobileTreeOpen ? (
              <div className="fixed inset-0 z-modal lg:hidden">
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
                        {selected ? getStorageEntityDisplayName(selected, t) : t("common.na")}
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
                    selected.entityType === "storage_location"
                      ? (hierarchy.repositoriesByLocation.get(selected.target_id ?? "") ?? [])
                      : []
                  }
                />
              ) : null}
            </section>
          </div>
        </div>
      )}
    </MonitorFrame>
  );
}

function SupportBundleButton() {
  const { t } = useI18n();
  const supportBundle = useStorageSupportBundle();
  const downloadSupportBundle = async () => {
    const result = await supportBundle.refetch();
    if (result.isError || !result.data) return;
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
    <div>
      <button
        type="button"
        className="btn btn-ghost btn-sm"
        disabled={supportBundle.isFetching}
        onClick={() => void downloadSupportBundle()}
      >
        <Download className="size-4" aria-hidden />
        {t("monitor.storage.download", "Download support bundle")}
      </button>
      {supportBundle.isError && (
        <p role="alert" className="text-xs text-error">
          {t("monitor.storage.bundleFailed", "Support bundle could not be downloaded.")}
        </p>
      )}
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
          const locationName = getStorageEntityDisplayName(location, t);
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
                aria-label={locationName}
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
                <span className="min-w-0 truncate">{locationName}</span>
                <StorageStatusDot item={location} className="ml-auto" />
              </button>
              {isOpen && repositories.length > 0 ? (
                <ul className="ml-4 border-l border-base-content/10 pl-2 lg:ml-3.5">
                  {repositories.map((repository) => {
                    const isRepoSelected = repository.target_id === selectedID;
                    const repositoryName = getStorageEntityDisplayName(repository, t);
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
                          <span className="min-w-0 truncate">{repositoryName}</span>
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
              const repositoryName = getStorageEntityDisplayName(repository, t);
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
                    <span className="min-w-0 truncate">{repositoryName}</span>
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

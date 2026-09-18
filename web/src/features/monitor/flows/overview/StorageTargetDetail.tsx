import { useState, type ReactNode } from "react";
import { Check, Copy, HardDrive } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { getStorageEntityDisplayName, type StorageDiagnostic } from "@/features/repositories";
import { copyText } from "@/lib/clipboard";
import { useI18n } from "@/lib/i18n";
import { CapacityMap } from "./CapacityMap";
import { MonitorDiagnostics } from "./MonitorFrame";
import {
  reachabilityLabel,
  riskLabel,
  storageItemSeverity,
  storageRiskKeys,
} from "../../model/storageSeverity";

export function StorageTargetDetail({
  item,
  repositories,
}: {
  item: StorageDiagnostic;
  repositories: StorageDiagnostic[];
}) {
  if (item.entityType === "storage_location") {
    return <StorageLocationDetail item={item} repositories={repositories} />;
  }

  return (
    <StorageInformationView item={item} titleMeta={<StorageTitleMeta item={item} />}>
      <div className="grid w-full min-w-0 content-start gap-3">
        <CapacityPanel item={item} />
        <StorageRiskPanels item={item} />
        <TechnicalDetailsPanel item={item} />
      </div>
    </StorageInformationView>
  );
}

function StorageInformationView({
  item,
  titleMeta,
  headerAside,
  children,
}: {
  item: StorageDiagnostic;
  titleMeta?: ReactNode;
  headerAside?: ReactNode;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const headingID = `storage-target-${item.target_id || "unknown"}`;
  const displayName = getStorageEntityDisplayName(item, t);

  return (
    <section aria-labelledby={headingID} className="h-auto w-full min-w-0 max-w-full bg-base-100">
      <div className="h-auto w-full min-w-0 max-w-full space-y-4">
        <header className="flex w-full min-w-0 items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h2 id={headingID} className="min-w-0 truncate text-base font-semibold">
                {displayName || t("common.na")}
              </h2>
              {titleMeta}
            </div>
            <p
              className="mt-1 block max-w-full truncate font-mono text-xs text-base-content/50"
              title={item.path}
            >
              {item.path || t("common.na")}
            </p>
          </div>
          {headerAside}
        </header>

        {children}
      </div>
    </section>
  );
}

function StorageLocationDetail({
  item,
  repositories,
}: {
  item: StorageDiagnostic;
  repositories: StorageDiagnostic[];
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [copiedRepositoryID, setCopiedRepositoryID] = useState<string | null>(null);

  const copyRepositoryPath = async (repository: StorageDiagnostic, repositoryKey: string) => {
    if (!repository.path) return;
    try {
      await copyText(repository.path);
      setCopiedRepositoryID(repositoryKey);
    } catch {
      setCopiedRepositoryID(null);
      showMessage("error", t("common.copyFailed", { defaultValue: "Copy failed." }));
    }
  };

  return (
    <StorageInformationView
      item={item}
      titleMeta={<StorageTitleMeta item={item} />}
      headerAside={
        <div className="shrink-0 text-right">
          <p className="text-base font-semibold tabular-nums">{repositories.length}</p>
          <p className="mt-1 font-mono text-xs text-base-content/50">
            {t("manage.repositories.title", "Repositories")}
          </p>
        </div>
      }
    >
      <CapacityPanel item={item} />
      <StorageRiskPanels item={item} />
      <div className="grid w-full min-w-0 items-start gap-4">
        <section className="flex h-auto min-h-28 min-w-0 max-h-[60dvh] w-full max-w-full flex-col rounded-box border border-base-300 bg-base-100 p-3 lg:h-full lg:min-h-0 lg:max-h-none">
          <SectionLabel>{t("manage.repositories.title", "Repositories")}</SectionLabel>
          <ul className="m-0 mt-2 flex h-auto min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-y-auto overscroll-contain border-0 bg-transparent p-0">
            {repositories.length === 0 ? (
              <li className="text-sm text-base-content/50">
                {t(
                  "monitor.storage.noRepositories",
                  "No Repositories are registered in this Storage Location.",
                )}
              </li>
            ) : (
              repositories.map((repository, index) => {
                const repositoryName = getStorageEntityDisplayName(repository, t);
                const repositoryKey =
                  repository.target_id ??
                  repository.path ??
                  repository.rawName ??
                  `repository-${index}`;
                const isCopied = copiedRepositoryID === repositoryKey;

                return (
                  <li key={repositoryKey} className="m-0 flex min-w-0 items-center gap-3 p-0">
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <div className="min-w-0 truncate text-sm font-medium">{repositoryName}</div>
                        <StorageStatusDot item={repository} />
                      </div>
                      <div
                        className="block max-w-full truncate font-mono text-[11px] text-base-content/45"
                        title={repository.path}
                      >
                        {repository.path || t("common.na")}
                      </div>
                    </div>
                    <button
                      type="button"
                      className="btn btn-square btn-ghost btn-sm shrink-0"
                      disabled={!repository.path}
                      aria-label={`${t("common.copy")} ${repositoryName || t("common.na")}`}
                      title={isCopied ? t("common.copied") : t("common.copy")}
                      onClick={() => void copyRepositoryPath(repository, repositoryKey)}
                    >
                      {isCopied ? (
                        <Check className="size-[1.2em] text-success" aria-hidden="true" />
                      ) : (
                        <Copy className="size-[1.2em]" aria-hidden="true" />
                      )}
                    </button>
                  </li>
                );
              })
            )}
          </ul>
        </section>

        <div className="grid w-full min-w-0 max-w-full content-start gap-3 self-start">
          <TechnicalDetailsPanel item={item} />
        </div>
      </div>
    </StorageInformationView>
  );
}

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <h3 className="text-[11px] font-semibold tracking-wider text-base-content/50 uppercase">
      {children}
    </h3>
  );
}

/** Reachability and writability, owned once for both detail bodies. */
function StorageTitleMeta({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  return (
    <>
      <StorageStatusBadge item={item} />
      {item.writable === false && (
        <span className="badge badge-ghost badge-sm">
          {t("monitor.storage.readOnly", "Read-only")}
        </span>
      )}
    </>
  );
}

function CapacityPanel({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  return (
    <section className="h-auto w-full min-w-0 max-w-full space-y-3">
      <SectionLabel>{t("monitor.storage.capacityHeading", "Capacity")}</SectionLabel>
      <CapacityMap key={item.target_id} item={item} />
    </section>
  );
}

function TechnicalDetailsPanel({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  return (
    <MonitorDiagnostics title={t("monitor.storage.technicalDetails", "Technical details")}>
      <TechnicalDetails item={item} />
      {/* 容量读数的告警：唯一无法从其他位置读出的容量事实，留在可展开的诊断里 */}
      <p className="text-xs text-base-content/60">
        {t(
          "monitor.storage.capacityNote",
          "The filesystem can report less writable space than macOS shows when purgeable APFS storage is included.",
        )}
      </p>
    </MonitorDiagnostics>
  );
}

function TechnicalDetails({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  const emptyValue = t("common.na");
  const facts = [
    {
      label: t("monitor.storage.filesystem", "Filesystem"),
      value: item.filesystem,
    },
    {
      label: t("monitor.storage.mount", "Mount"),
      value: item.mount_id || item.device,
      mono: true,
    },
    {
      label: t("monitor.storage.marker", "Marker identity"),
      value: item.marker_uuid,
      mono: true,
    },
    {
      label: t("monitor.storage.lockHolder", "Lock holder"),
      value: item.lock_holder,
      mono: true,
    },
    {
      label: t("monitor.storage.effectiveIdentity", "Effective identity"),
      value:
        item.effective_uid || item.effective_gid
          ? `${item.effective_uid || emptyValue}:${item.effective_gid || emptyValue}`
          : undefined,
      mono: true,
    },
    {
      label: t("monitor.storage.caseBehavior", "Case behavior"),
      value: item.case_behavior_known
        ? item.case_sensitive
          ? t("monitor.storage.caseSensitive", "Case-sensitive")
          : t("monitor.storage.caseInsensitive", "Case-insensitive")
        : undefined,
    },
  ];

  return (
    <dl className="mt-2 w-full min-w-0 divide-y divide-base-300/70 text-xs">
      {facts.map((fact) => (
        <div
          key={fact.label}
          className="flex min-w-0 items-center justify-between gap-4 py-2 first:pt-0 last:pb-0"
        >
          <dt className="shrink-0 text-[11px] text-base-content/45">{fact.label}</dt>
          <dd
            className={`ml-auto min-w-0 max-w-[65%] truncate text-right ${fact.mono ? "font-mono text-[11px]" : ""}`}
            title={fact.value}
          >
            {fact.value || emptyValue}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function StorageRiskPanels({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  const risks = storageRiskKeys(item);

  if (risks.length === 0) return null;

  return (
    <div className="mt-2 space-y-1.5">
      {risks.map((risk) => (
        <div
          key={risk}
          className="flex min-w-0 items-center gap-2 rounded-field bg-base-200/60 px-3 py-2 text-xs font-medium text-primary"
        >
          <HardDrive className="size-4 shrink-0" strokeWidth={1.7} aria-hidden="true" />
          <span className="min-w-0">{riskLabel(t, risk)}</span>
        </div>
      ))}
    </div>
  );
}

export function StorageStatusDot({
  item,
  className = "",
}: {
  item: StorageDiagnostic;
  className?: string;
}) {
  const severity = storageItemSeverity(item);
  const color =
    severity === "healthy" ? "bg-success" : severity === "warning" ? "bg-warning" : "bg-error";
  return (
    <span aria-hidden="true" className={`size-2 shrink-0 rounded-full ${color} ${className}`} />
  );
}

function StorageStatusBadge({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  const severity = storageItemSeverity(item);
  const tone =
    severity === "healthy"
      ? "badge-primary"
      : severity === "warning"
        ? "badge-warning"
        : "badge-error";
  return (
    <span className={`badge badge-sm badge-soft ${tone} shrink-0`}>
      {reachabilityLabel(t, item.reachability)}
    </span>
  );
}

import { useState, type CSSProperties, type ReactNode } from "react";
import { Check, Copy, HardDrive } from "lucide-react";
import { useMessage } from "@/features/notifications";
import type { components } from "@/lib/http-commons/schema";
import { copyText } from "@/lib/clipboard";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";

type StorageDiagnostic = components["schemas"]["dto.StorageDiagnosticDTO"];

export function StorageTargetDetail({
  item,
  repositories,
}: {
  item: StorageDiagnostic;
  repositories: StorageDiagnostic[];
}) {
  if (item.target_type === "storage_location") {
    return <StorageLocationDetail item={item} repositories={repositories} />;
  }

  return (
    <StorageInformationView item={item}>
      <div className="grid w-full min-w-0 content-start gap-3">
        <CapacityPanel item={item} />
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

  return (
    <section
      aria-labelledby={headingID}
      className="card h-auto w-full min-w-0 max-w-full rounded-box border border-base-300 bg-base-100 shadow-sm"
    >
      <div className="card-body h-auto w-full min-w-0 max-w-full gap-3 p-4 sm:p-5">
        <header className="flex w-full min-w-0 items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h2 id={headingID} className="min-w-0 truncate text-base font-semibold">
                {item.name || t("common.na")}
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
      titleMeta={
        <>
          <StorageStatusBadge item={item} />
          {item.writable === false ? (
            <span className="badge badge-ghost badge-sm">
              {t("monitor.storage.readOnly", "Read-only")}
            </span>
          ) : null}
        </>
      }
      headerAside={
        <div className="shrink-0 text-right">
          <p className="text-base font-semibold tabular-nums">{repositories.length}</p>
          <p className="mt-1 font-mono text-xs text-base-content/50">
            {t("manage.repositories.title", "Repositories")}
          </p>
        </div>
      }
    >
      <div className="grid w-full min-w-0 items-start gap-3 lg:grid-cols-[minmax(0,1.06fr)_minmax(0,0.94fr)] lg:items-stretch">
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
                const repositoryKey =
                  repository.target_id ??
                  repository.path ??
                  repository.name ??
                  `repository-${index}`;
                const isCopied = copiedRepositoryID === repositoryKey;

                return (
                  <li key={repositoryKey} className="m-0 flex min-w-0 items-center gap-3 p-0">
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <div className="min-w-0 truncate text-sm font-medium">
                          {repository.name}
                        </div>
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
                      aria-label={`${t("common.copy")} ${repository.name || t("common.na")}`}
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
          <CapacityPanel item={item} />
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

function CapacityPanel({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  return (
    <section className="h-auto w-full min-w-0 max-w-full rounded-box border border-base-300 bg-base-100 p-3">
      <SectionLabel>{t("monitor.storage.capacityHeading", "Capacity")}</SectionLabel>
      <CapacityDonut item={item} />
    </section>
  );
}

function TechnicalDetailsPanel({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  return (
    <section className="h-auto w-full min-w-0 max-w-full rounded-box border border-base-300 bg-base-100 p-4">
      <SectionLabel>{t("monitor.storage.technicalDetails", "Technical details")}</SectionLabel>
      <TechnicalDetails item={item} />
      <StorageRiskPanels item={item} />
    </section>
  );
}

function CapacityDonut({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  if (!item.capacity_known) {
    return (
      <p className="mt-2 text-center text-sm text-base-content/50">
        {t("monitor.storage.capacityUnknown", "Capacity unavailable")}
      </p>
    );
  }

  const usedPercent = Math.round(capacityUsedPercent(item));
  const usedBytes = Math.max(0, (item.total_bytes ?? 0) - (item.available_bytes ?? 0));
  const ringTone =
    usedPercent >= 90 ? "text-error" : usedPercent >= 80 ? "text-warning" : "text-primary";

  return (
    <div className="mx-auto mt-2 flex w-full min-w-0 max-w-xl flex-wrap items-center justify-center gap-4">
      <div
        role="progressbar"
        aria-label={t("monitor.storage.capacityUsed", "Used capacity")}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={usedPercent}
        className={`radial-progress shrink-0 text-sm font-semibold tabular-nums ${ringTone}`}
        style={
          {
            "--value": usedPercent,
            "--size": "5.5rem",
            "--thickness": "0.55rem",
          } as CSSProperties
        }
      >
        {usedPercent}%
      </div>

      <div className="min-w-0 space-y-2 text-xs">
        <CapacityLegend
          color="bg-primary"
          label={t("monitor.storage.statUsed", "Used")}
          value={formatBytes(usedBytes)}
        />
        <CapacityLegend
          color="bg-base-300"
          label={t("monitor.storage.statAvailable", "Available")}
          value={formatBytes(item.available_bytes ?? 0)}
        />
        <p className="tabular-nums text-base-content/50">
          {t("monitor.storage.totalCapacity", "Total {{total}}", {
            total: formatBytes(item.total_bytes ?? 0),
          })}
        </p>
      </div>
    </div>
  );
}

function CapacityLegend({ color, label, value }: { color: string; label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-center gap-1.5 text-base-content/65">
      <span className={`size-2.5 shrink-0 rounded-full ${color}`} aria-hidden="true" />
      <span className="truncate">
        {label} <span className="tabular-nums">{value}</span>
      </span>
    </div>
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
  const risks = new Set(item.risk_warnings ?? []);
  if (item.removable_likely) risks.add("removable_storage");
  if (item.network_filesystem) risks.add("network_filesystem");
  if (item.cloud_sync_provider) risks.add("cloud_sync_directory");
  if (item.mount_fingerprint_changed) risks.add("mount_fingerprint_changed");

  if (risks.size === 0) return null;

  return (
    <div className="mt-2 space-y-1.5">
      {[...risks].map((risk) => (
        <div
          key={risk}
          className="flex min-w-0 items-center gap-2 rounded-field bg-base-200/60 px-3 py-2 text-xs font-medium text-primary"
        >
          <HardDrive className="size-4 shrink-0" strokeWidth={1.7} aria-hidden="true" />
          <span className="min-w-0">{riskLabel(risk, t)}</span>
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
      {reachabilityLabel(item.reachability, t)}
    </span>
  );
}

export function storageItemSeverity(item: StorageDiagnostic): "healthy" | "warning" | "error" {
  if (
    item.reachability === "offline" ||
    item.reachability === "identity_error" ||
    item.reachability === "recovery_required"
  ) {
    return "error";
  }
  if (!isStorageHealthy(item) || (item.risk_warnings ?? []).length > 0) {
    return "warning";
  }
  return "healthy";
}

function isStorageHealthy(item: StorageDiagnostic): boolean {
  return item.reachability === "active" && item.writable !== false;
}

function capacityUsedPercent(item: StorageDiagnostic): number {
  if (!item.capacity_known || !item.total_bytes || item.total_bytes <= 0) return 0;
  const available = Math.max(0, item.available_bytes ?? 0);
  return Math.min(100, Math.max(0, ((item.total_bytes - available) / item.total_bytes) * 100));
}

function reachabilityLabel(value: string | undefined, t: ReturnType<typeof useI18n>["t"]): string {
  switch (value) {
    case "active":
      return t("monitor.storage.statusActive", "Available");
    case "maintenance":
      return t("monitor.storage.statusMaintenance", "Maintenance");
    case "offline":
      return t("monitor.storage.statusOffline", "Offline");
    case "identity_error":
      return t("monitor.storage.statusIdentityError", "Identity mismatch");
    case "recovery_required":
      return t("monitor.storage.statusRecoveryRequired", "Recovery required");
    default:
      return value || t("monitor.storage.statusUnknown", "Unknown");
  }
}

function riskLabel(risk: string, t: ReturnType<typeof useI18n>["t"]): string {
  const labels: Record<string, string> = {
    removable_storage: t("monitor.storage.riskRemovable", "Removable storage"),
    network_filesystem: t("monitor.storage.riskNetwork", "Network filesystem"),
    cloud_sync_directory: t("monitor.storage.riskCloudSync", "Cloud-sync directory"),
    mount_fingerprint_changed: t("monitor.storage.riskMountChanged", "Mount identity changed"),
  };
  return labels[risk] ?? risk;
}

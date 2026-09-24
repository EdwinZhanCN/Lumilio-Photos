import { useMemo } from "react";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import { useStorageDiagnostics } from "../../api/useStorageDiagnostics";
import type { RepositoryRow } from "./storageViewModel";
import { storageStateLabel, verificationLabel } from "./storageStateCopy";

type Props = {
  repository: RepositoryRow;
  /** Display name resolved by the caller, so product terms stay in one place. */
  displayName: string;
};

type Fact = { term: string; value: string };

/**
 * The facts for one Repository, opened inline beneath its row. One level and
 * one visual form: every fact is plain text, in a single column on narrow
 * screens and two columns from `sm` up. Capacity is a sentence that names its
 * storage once — never a gauge beside a repeated number.
 */
export default function RepositoryInspector({ repository, displayName }: Props) {
  const { t } = useI18n();
  const diagnostics = useStorageDiagnostics(true);

  const diagnostic = useMemo(
    () =>
      (diagnostics.data?.items ?? []).find(
        (item) => item.entityType === "repository" && item.target_id === repository.id,
      ),
    [diagnostics.data?.items, repository.id],
  );

  const storage = [
    repository.storage.kind === "named"
      ? repository.storage.name
      : repository.storage.kind === "system"
        ? t("storagePanel.storage.system", "Built-in storage")
        : t("storagePanel.storage.unknown", "Storage not resolved"),
    repository.filesystem,
  ]
    .filter(Boolean)
    .join(" · ");

  const facts: Fact[] = [
    { term: t("storagePanel.inspector.repository", "Repository"), value: displayName },
    { term: t("storagePanel.inspector.storage", "Storage"), value: storage },
    {
      term: t("storagePanel.inspector.available", "Capacity"),
      value: repository.capacity?.capacityKnown
        ? t("storagePanel.capacityValue", "{{available}} available of {{total}}", {
            available: formatBytes(repository.capacity.availableBytes ?? 0),
            total: formatBytes(repository.capacity.totalBytes ?? 0),
          })
        : t("storagePanel.capacityUnknownShort", "Capacity is unknown right now."),
    },
    {
      term: t("storagePanel.inspector.state", "State"),
      value: storageStateLabel(t, repository.state),
    },
    { term: t("storagePanel.inspector.assets", "Assets"), value: String(repository.assetCount) },
    {
      term: t("storagePanel.inspector.verification", "Scan"),
      value: verificationLabel(t, repository.verification),
    },
  ];

  if (repository.storage.kind !== "unknown" && repository.storage.mountPath) {
    facts.push({
      term: t("storagePanel.inspector.mountPath", "Mount point"),
      value: repository.storage.mountPath,
    });
  }

  if (diagnostic) {
    facts.push(
      {
        term: t("storagePanel.technical.mount", "Mount"),
        value: diagnostic.mount_id || diagnostic.device || "—",
      },
      {
        term: t("storagePanel.technical.effectiveIdentity", "Effective identity"),
        value:
          diagnostic.effective_uid || diagnostic.effective_gid
            ? `${diagnostic.effective_uid || "—"}:${diagnostic.effective_gid || "—"}`
            : "—",
      },
      {
        term: t("storagePanel.technical.caseBehavior", "Case behavior"),
        value: diagnostic.case_behavior_known
          ? diagnostic.case_sensitive
            ? t("storagePanel.technical.caseSensitive", "Case-sensitive")
            : t("storagePanel.technical.caseInsensitive", "Case-insensitive")
          : "—",
      },
      {
        term: t("storagePanel.technical.marker", "Marker identity"),
        value: diagnostic.marker_uuid || "—",
      },
      {
        term: t("storagePanel.technical.lockHolder", "Lock holder"),
        value: diagnostic.lock_holder || "—",
      },
    );
  }

  return (
    <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-1.5 px-4 py-3 sm:grid-cols-2">
      {facts.map((fact) => (
        <div key={fact.term} className="flex min-w-0 items-baseline gap-3">
          <dt className="w-28 shrink-0 text-xs text-base-content/55">{fact.term}</dt>
          <dd className="m-0 min-w-0 break-words text-xs text-base-content/85">{fact.value}</dd>
        </div>
      ))}
    </dl>
  );
}

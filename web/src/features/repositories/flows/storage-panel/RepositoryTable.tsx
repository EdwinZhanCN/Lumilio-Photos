import { Fragment, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import { getStorageEntityDisplayName } from "../../model/storageEntities";
import RepositoryInspector from "./RepositoryInspector";
import type { RepositoryRow } from "./storageViewModel";
import {
  storageStateBadgeClass,
  storageStateLabel,
  verificationBadgeClass,
  verificationLabel,
} from "./storageStateCopy";

type Props = {
  rows: RepositoryRow[];
  expandedId: string | null;
  onToggleRow: (id: string) => void;
  /** Row commands, rendered in the trailing cell. */
  renderActions: (row: RepositoryRow) => ReactNode;
};

const COLUMN_COUNT = 7;

/**
 * Repository rows for one Storage Location. The model's own order is the
 * reading order: no sorting, filtering, or search. Selecting a row opens its
 * facts inline beneath it.
 */
export default function RepositoryTable({ rows, expandedId, onToggleRow, renderActions }: Props) {
  const { t } = useI18n();

  const headers: { label: string; className?: string }[] = [
    { label: t("storagePanel.column.name", "Repository") },
    { label: t("storagePanel.column.storage", "Storage") },
    { label: t("storagePanel.column.available", "Available") },
    { label: t("storagePanel.column.state", "State") },
    { label: t("storagePanel.column.assets", "Assets") },
    { label: t("storagePanel.column.verification", "Scan") },
    { label: t("storagePanel.column.actions", "Actions"), className: "text-right" },
  ];

  return (
    <div className="overflow-x-auto">
      <table className="table table-sm">
        <thead>
          <tr>
            {headers.map((header) => (
              <th key={header.label} scope="col" className={header.className}>
                {header.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const expanded = expandedId === row.id;
            const displayName = getStorageEntityDisplayName(
              { entityType: "repository", role: row.role, rawName: row.name, path: "" },
              t,
            );

            return (
              <Fragment key={row.id}>
                <tr
                  className={`cursor-pointer hover:bg-base-200/40 ${
                    expanded ? "bg-base-200/40" : ""
                  }`}
                  onClick={(event) => {
                    // Interactive children own their own clicks.
                    if ((event.target as HTMLElement).closest("button, a, input, [role='menu']")) {
                      return;
                    }
                    onToggleRow(row.id);
                  }}
                >
                  <th scope="row" className="font-normal">
                    {/* No expander icon: the whole row is the target, cued by the
                        pointer cursor and hover background. The button keeps the
                        keyboard path and the expanded state announcement. */}
                    <button
                      type="button"
                      className="flex w-full min-w-0 cursor-pointer items-center text-left"
                      aria-expanded={expanded}
                      onClick={(event) => {
                        event.stopPropagation();
                        onToggleRow(row.id);
                      }}
                    >
                      <span className="truncate text-sm font-medium">{displayName}</span>
                    </button>
                  </th>
                  <td className="whitespace-nowrap text-xs text-base-content/75">
                    {row.storage.kind === "named"
                      ? row.storage.name
                      : row.storage.kind === "system"
                        ? t("storagePanel.storage.system", "Built-in storage")
                        : t("storagePanel.storage.unknown", "Storage not resolved")}
                  </td>
                  <td className="whitespace-nowrap text-xs tabular-nums text-base-content/75">
                    {row.capacity?.capacityKnown
                      ? formatBytes(row.capacity.availableBytes ?? 0)
                      : t("storagePanel.capacityUnknownShort", "Capacity is unknown right now.")}
                  </td>
                  <td>
                    <span
                      className={`badge badge-sm badge-soft ${storageStateBadgeClass(row.state)}`}
                    >
                      {storageStateLabel(t, row.state)}
                    </span>
                  </td>
                  <td className="text-xs tabular-nums text-base-content/75">{row.assetCount}</td>
                  <td>
                    <span
                      className={`badge badge-sm badge-soft ${verificationBadgeClass(row.verification)}`}
                    >
                      {verificationLabel(t, row.verification)}
                    </span>
                  </td>
                  <td className="text-right">{renderActions(row)}</td>
                </tr>
                {expanded ? (
                  <tr className="bg-base-200/30">
                    <td colSpan={COLUMN_COUNT} className="p-0">
                      <RepositoryInspector repository={row} displayName={displayName} />
                    </td>
                  </tr>
                ) : null}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

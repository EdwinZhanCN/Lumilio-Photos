import type { ReactNode } from "react";
import { FolderPlus, FolderSearch, HardDrive, Trash2 } from "lucide-react";
import AnchoredMenu, { MenuItem } from "@/components/ui/AnchoredMenu";
import { localizeStorageActionBlockers } from "@/lib/http-commons/problem";
import { useI18n } from "@/lib/i18n";
import { getStorageEntityDisplayName } from "../../model/storageEntities";
import RepositoryTable from "./RepositoryTable";
import type { RepositoryRow, StorageLocationGroup } from "./storageViewModel";

type Props = {
  group: StorageLocationGroup;
  expandedId: string | null;
  onToggleRow: (id: string) => void;
  renderActions: (row: RepositoryRow) => ReactNode;
  onAddRepository: () => void;
  onRemoveLocation: () => void;
  /** The Location Reconnect entry needs a Desktop host to choose the folder. */
  nativeHostAvailable?: boolean;
  onReconnectLocation?: () => void;
};

/**
 * One Storage Location section. The header carries identity and Location-level
 * commands only: capacity is never shown here, because a Location can hold
 * Repositories on different mounts and a single figure would misattribute at
 * least one of them. On a Desktop host an external Location also carries its
 * direct Reconnect entry; while the Server withholds it, the entry stays
 * visible but disabled with the Server's reason, so the prerequisite (the
 * original is positively unavailable) is never a mystery.
 */
export default function StorageLocationSection({
  group,
  expandedId,
  onToggleRow,
  renderActions,
  onAddRepository,
  onRemoveLocation,
  nativeHostAvailable = false,
  onReconnectLocation,
}: Props) {
  const { t } = useI18n();
  const reconnectAllowed = group.actions.includes("reconnect");
  const reconnectBlock = group.unavailableActions.find((block) => block.action === "reconnect");
  const showReconnect = nativeHostAvailable && group.kind === "external" && onReconnectLocation;
  const displayName = getStorageEntityDisplayName(
    { entityType: "storage_location", kind: group.kind, rawName: group.name, path: "" },
    t,
  );
  const isEmpty = group.repositories.length === 0;

  return (
    <section
      aria-label={displayName}
      className="overflow-hidden rounded-box border border-base-300 bg-base-100"
    >
      {/* Horizontal padding matches the table cell padding (12px) so the title
          lines up with the first column and both overflow buttons share one
          right edge. */}
      <header className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-3 py-3">
        <HardDrive className="size-4 shrink-0 text-base-content/50" strokeWidth={1.6} aria-hidden />
        <h2 className="min-w-0 truncate text-sm font-semibold">{displayName}</h2>
        {group.kind === "external" ? (
          <span className="badge badge-sm badge-soft badge-ghost">
            {t("storagePanel.kind.external", "External")}
          </span>
        ) : null}
        <span className="ml-auto shrink-0 text-xs tabular-nums text-base-content/50">
          {t("storagePanel.repositoryCount", "{{count}} Repositories", {
            count: group.repositories.length,
          })}
        </span>
        <AnchoredMenu
          label={t("storagePanel.locationActions", "Actions for {{name}}", {
            name: displayName,
          })}
          menuWidth={256}
          menuHeight={180}
          triggerClassName="btn btn-square btn-ghost btn-xs shrink-0"
        >
          {({ close }) => (
            <>
              <MenuItem
                onSelect={() => {
                  close();
                  onAddRepository();
                }}
              >
                <FolderPlus className="size-4" aria-hidden />
                {t("storagePanel.action.addRepository", "Add Repository")}
              </MenuItem>
              {showReconnect ? (
                <MenuItem
                  disabled={!reconnectAllowed}
                  hint={
                    reconnectAllowed
                      ? t(
                          "storagePanel.action.reconnectLocationHint",
                          "Choose the folder that holds this Storage Location and its Repositories now; nothing is moved.",
                        )
                      : localizeStorageActionBlockers(reconnectBlock?.blockers, t)
                  }
                  onSelect={() => {
                    close();
                    onReconnectLocation();
                  }}
                >
                  <FolderSearch className="size-4" aria-hidden />
                  {t("storagePanel.action.reconnectLocation", "Reconnect Storage Location")}
                </MenuItem>
              ) : null}
              <MenuItem
                danger
                disabled={!group.canRemove}
                // The reason rides on this entry, not on a row of its own.
                hint={
                  group.canRemove
                    ? undefined
                    : t("storagePanel.removeBlocked", "Remove its Repositories first.")
                }
                onSelect={() => {
                  close();
                  onRemoveLocation();
                }}
              >
                <Trash2 className="size-4" aria-hidden />
                {t("storagePanel.action.removeLocation", "Remove Storage Location")}
              </MenuItem>
            </>
          )}
        </AnchoredMenu>
      </header>

      <div className="border-t border-base-200">
        {isEmpty ? (
          <p className="m-0 px-3 py-6 text-center text-xs text-base-content/50">
            {t(
              "storagePanel.emptyLocationFacts",
              "Registered Storage Location with no Repositories. Detaching removes the registration only; nothing on disk is deleted.",
            )}
          </p>
        ) : (
          <RepositoryTable
            rows={group.repositories}
            expandedId={expandedId}
            onToggleRow={onToggleRow}
            renderActions={renderActions}
          />
        )}
      </div>
    </section>
  );
}

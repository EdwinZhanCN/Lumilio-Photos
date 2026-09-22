import type { ReactNode } from "react";
import {
  Cloud,
  Copy,
  FolderSearch,
  History,
  Layers,
  MapPin,
  Pencil,
  ScanSearch,
  Trash2,
  XCircle,
} from "lucide-react";
import AnchoredMenu, { MenuItem } from "@/components/ui/AnchoredMenu";
import { useI18n } from "@/lib/i18n";
import type { RepositoryRow } from "./storageViewModel";

export type RepositoryCommand =
  | "verify"
  | "cancelVerification"
  | "verificationHistory"
  | "detectStacks"
  | "detectDuplicates"
  | "rebuildLocations"
  | "rename"
  | "remove"
  | "cloudSources"
  | "locate";

type Props = {
  row: RepositoryRow;
  onCommand: (command: RepositoryCommand, row: RepositoryRow) => void;
  /** Native Folder reveal is only offered when a Desktop host can serve it. */
  nativeHostAvailable: boolean;
  isScanning: boolean;
};

/** Row commands. Destructive entries sit last and are the only colored item. */
export default function RepositoryRowActions({
  row,
  onCommand,
  nativeHostAvailable,
  isScanning,
}: Props) {
  const { t } = useI18n();

  return (
    <AnchoredMenu
      label={t("storagePanel.actionsFor", "Actions for {{name}}", { name: row.name })}
      menuWidth={240}
      menuHeight={380}
    >
      {({ close }) => {
        const item = (command: RepositoryCommand, icon: ReactNode, label: string) => (
          <MenuItem
            onSelect={() => {
              close();
              onCommand(command, row);
            }}
          >
            {icon}
            {label}
          </MenuItem>
        );

        return (
          <>
            <li role="none" className="menu-title">
              {t("storagePanel.menuVerification", "Scan")}
            </li>
            {isScanning
              ? item(
                  "cancelVerification",
                  <XCircle className="size-4" aria-hidden />,
                  t("storagePanel.action.cancelVerification", "Cancel scan"),
                )
              : item(
                  "verify",
                  <ScanSearch className="size-4" aria-hidden />,
                  t("storagePanel.action.scan", "Scan now"),
                )}
            {item(
              "verificationHistory",
              <History className="size-4" aria-hidden />,
              t("storagePanel.action.verificationHistory", "Scan history"),
            )}

            <li role="none" className="menu-title">
              {t("storagePanel.maintenance", "Maintenance")}
            </li>
            {item(
              "detectStacks",
              <Layers className="size-4" aria-hidden />,
              t("storagePanel.action.stacks", "Detect stacks"),
            )}
            {item(
              "detectDuplicates",
              <Copy className="size-4" aria-hidden />,
              t("storagePanel.action.detectDuplicates", "Detect duplicates"),
            )}
            {item(
              "rebuildLocations",
              <MapPin className="size-4" aria-hidden />,
              t("storagePanel.action.rebuild", "Rebuild location"),
            )}
            {nativeHostAvailable
              ? item(
                  "locate",
                  <FolderSearch className="size-4" aria-hidden />,
                  t("storagePanel.action.locate", "Locate on disk"),
                )
              : null}

            <li role="none" className="menu-title">
              {t("storagePanel.menuRepository", "Repository")}
            </li>
            {item(
              "rename",
              <Pencil className="size-4" aria-hidden />,
              t("storagePanel.action.rename", "Rename Repository"),
            )}
            {item(
              "cloudSources",
              <Cloud className="size-4" aria-hidden />,
              t("storagePanel.action.cloudSources", "Cloud sources"),
            )}

            {row.role === "primary" ? null : (
              <>
                <li role="none" className="menu-title text-error">
                  {t("storagePanel.menuDanger", "Danger zone")}
                </li>
                {item(
                  "remove",
                  <Trash2 className="size-4" aria-hidden />,
                  t("storagePanel.action.remove", "Remove from Lumilio"),
                )}
              </>
            )}
          </>
        );
      }}
    </AnchoredMenu>
  );
}

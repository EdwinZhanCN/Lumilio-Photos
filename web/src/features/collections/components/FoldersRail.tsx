import { FolderTree } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import Rail from "./Rail";
import RailCard from "./RailCard";
import type { FolderSummary } from "../hooks/useFolders";

type FoldersRailProps = {
  folders: FolderSummary[];
  loading?: boolean;
  onFolderClick?: (folder: FolderSummary) => void;
};

export default function FoldersRail({ folders, loading = false, onFolderClick }: FoldersRailProps) {
  const { t } = useI18n();

  return (
    <Rail
      loading={loading}
      isEmpty={folders.length === 0}
      empty={
        <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-8 text-sm text-base-content/60">
          {t("collections.emptyFolders")}
        </div>
      }
    >
      {folders.map((folder) => (
        <RailCard
          key={`${folder.repository_id}:${folder.folder_path}`}
          media={{
            kind: "photo",
            src: folder.cover_asset_id
              ? assetUrls.getThumbnailUrl(folder.cover_asset_id, "medium")
              : null,
            fallbackIcon: FolderTree,
          }}
          title={folder.display_name || folder.folder_path || ""}
          subtitle={[
            folder.repository_name,
            t("collections.folders.itemCount", {
              count: folder.asset_count ?? 0,
              defaultValue: "{{count}} items",
            }),
          ]
            .filter(Boolean)
            .join(" · ")}
          onClick={() => onFolderClick?.(folder)}
          className="w-48"
        />
      ))}
    </Rail>
  );
}

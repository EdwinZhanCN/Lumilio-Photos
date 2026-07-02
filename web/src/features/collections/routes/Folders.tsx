import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { FolderTree } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import BrowseScopeSelect from "@/components/BrowseScopeSelect";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { useBrowseScope } from "@/features/settings";
import { assetUrls } from "@/lib/assets/assetUrls";
import RailCard from "../components/RailCard";
import { useFolders, type FolderSummary } from "../hooks/useFolders";
import { encodeFolderKey } from "../utils/folderKey";
import { formatDateRange } from "../utils/formatDateRange";

function FoldersContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { scopedRepositoryId } = useBrowseScope();
  const { data, isPending } = useFolders(scopedRepositoryId, "");
  const folders = data?.folders ?? [];

  const groups = useMemo(() => {
    const map = new Map<
      string,
      { repositoryId: string; repositoryName: string; folders: FolderSummary[] }
    >();
    for (const folder of folders) {
      const repositoryId = folder.repository_id ?? "";
      const existing = map.get(repositoryId);
      if (existing) {
        existing.folders.push(folder);
      } else {
        map.set(repositoryId, {
          repositoryId,
          repositoryName: folder.repository_name ?? repositoryId,
          folders: [folder],
        });
      }
    }
    return Array.from(map.values());
  }, [folders]);

  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.folders", "Folders") },
  ]);

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.folders", "Folders")}
        icon={<FolderTree className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
      </PageHeader>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        {!isPending && folders.length === 0 ? (
          <p className="text-sm text-base-content/60">
            {t("collections.folders.empty", "No folders found in this repository yet.")}
          </p>
        ) : (
          <div className="space-y-6">
            {groups.map((group) => (
              <section key={group.repositoryId} className="space-y-3">
                <div className="flex items-center gap-2 px-1">
                  <FolderTree className="size-4 text-base-content/50" strokeWidth={1.5} />
                  <h2 className="text-base font-semibold text-base-content/70">
                    {group.repositoryName}
                  </h2>
                </div>
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                  {group.folders.map((folder) => (
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
                        t("collections.folders.itemCount", {
                          count: folder.asset_count ?? 0,
                          defaultValue: "{{count}} items",
                        }),
                        formatDateRange(folder.date_start, folder.date_end),
                      ]
                        .filter(Boolean)
                        .join(" · ")}
                      onClick={() =>
                        navigate(
                          `/collections/folders/${encodeFolderKey({
                            repositoryId: folder.repository_id ?? "",
                            folderPath: folder.folder_path ?? "",
                          })}`,
                        )
                      }
                      className="w-full"
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default function Folders() {
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
      <FoldersContent />
    </ErrorBoundary>
  );
}

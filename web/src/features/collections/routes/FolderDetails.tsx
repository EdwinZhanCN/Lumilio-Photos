import { Navigate, useNavigate, useParams } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { useMemo } from "react";
import { FolderTree, Folder as FolderIcon } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetsProvider } from "@/features/assets";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import type { AssetFilter } from "@/features/assets/types/assets.type";
import { useFolders, useFolderSummary } from "../hooks/useFolders";
import { decodeFolderKey, encodeFolderKey } from "../utils/folderKey";

export default function FolderDetails() {
  const { folderKey } = useParams<{ folderKey: string }>();
  const navigate = useNavigate();
  const { t } = useI18n();
  const identity = useMemo(() => decodeFolderKey(folderKey), [folderKey]);

  const summaryQuery = useFolderSummary(identity?.repositoryId, identity?.folderPath ?? "");
  const childrenQuery = useFolders(identity?.repositoryId, identity?.folderPath ?? "");
  const summary = summaryQuery.data;
  const children = childrenQuery.data?.folders ?? [];

  const baseFilter: AssetFilter = useMemo(
    () => ({
      repository_id: identity?.repositoryId,
      folder_path: identity?.folderPath ?? "",
      folder_recursive: true,
    }),
    [identity?.repositoryId, identity?.folderPath],
  );

  const pathSegments = useMemo(() => {
    if (!identity || identity.folderPath === "") return [];
    const parts = identity.folderPath.split("/");
    return parts.map((part, index) => ({
      label: part,
      folderPath: parts.slice(0, index + 1).join("/"),
    }));
  }, [identity]);

  useBreadcrumbs(
    identity
      ? [
          { label: t("sidebar.home", "Home"), to: "/" },
          { label: t("sidebar.collections", "Collections"), to: "/collections" },
          { label: t("collections.sections.folders", "Folders"), to: "/collections/folders" },
          ...pathSegments.map((segment, index) => ({
            label: segment.label,
            to:
              index === pathSegments.length - 1
                ? undefined
                : `/collections/folders/${encodeFolderKey({
                    repositoryId: identity.repositoryId,
                    folderPath: segment.folderPath,
                  })}`,
          })),
        ]
      : [],
  );

  if (!identity) {
    return <Navigate to="/collections/folders" replace />;
  }

  const basePath = `/collections/folders/${folderKey}`;
  const title = summary?.display_name || identity.folderPath || t("collections.folders.root", "Root");

  const hero = (
    <div className="flex flex-col gap-2 px-4 pb-2">
      {summary && (
        <p className="text-sm text-base-content/60">
          {[
            summary.repository_name,
            t("collections.folders.itemCount", {
              count: summary.asset_count ?? 0,
              defaultValue: "{{count}} items",
            }),
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      )}
      {children.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {children.map((child) => (
            <button
              key={child.folder_path}
              type="button"
              className="btn btn-sm btn-soft btn-info gap-1.5"
              onClick={() =>
                navigate(
                  `/collections/folders/${encodeFolderKey({
                    repositoryId: child.repository_id ?? identity.repositoryId,
                    folderPath: child.folder_path ?? "",
                  })}`,
                )
              }
            >
              <FolderIcon className="h-3.5 w-3.5" />
              {child.display_name}
              <span className="opacity-60">({child.asset_count ?? 0})</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );

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
      <AssetsProvider scopeId={`collections:folders:${folderKey}`} syncUrl basePath={basePath}>
        <WorkerProvider>
          <AssetsGalleryPage
            title={title}
            icon={<FolderTree className="w-6 h-6 text-primary" />}
            baseFilter={baseFilter}
            viewKey={`folder:${identity.repositoryId}:${identity.folderPath}`}
            hero={hero}
          />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
}

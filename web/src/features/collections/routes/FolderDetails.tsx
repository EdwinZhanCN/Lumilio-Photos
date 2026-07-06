import { Navigate, useNavigate, useParams } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { useMemo } from "react";
import { FolderTree, Folder as FolderIcon } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetsProvider } from "@/features/assets";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { CollectionHero, MetaStat } from "@/components/collection";
import { useI18n } from "@/lib/i18n";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { AssetFilter } from "@/features/assets/types/assets.type";
import { useFolders, useFolderSummary } from "../hooks/useFolders";
import { decodeFolderKey, encodeFolderKey } from "../utils/folderKey";
import { formatDateRange } from "../utils/formatDateRange";

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
  const title =
    summary?.display_name || identity.folderPath || t("collections.folders.root", "Root");
  const isSummaryLoading = summaryQuery.isPending && !summary;
  const dateRangeLabel = formatDateRange(summary?.date_start, summary?.date_end);

  const hero = (
    <CollectionHero
      loading={isSummaryLoading}
      title={title}
      stats={
        <>
          <MetaStat loading={isSummaryLoading}>{summary?.repository_name}</MetaStat>
          <MetaStat loading={isSummaryLoading}>
            {t("collections.folders.itemCount", {
              count: summary?.asset_count ?? 0,
              defaultValue: "{{count}} items",
            })}
          </MetaStat>
          {dateRangeLabel && <MetaStat>{dateRangeLabel}</MetaStat>}
        </>
      }
      footer={
        children.length > 0 && (
          <div className="mt-4 flex items-center gap-2 overflow-x-auto pb-1">
            {children.map((child) => (
              <button
                key={child.folder_path}
                type="button"
                className="btn btn-sm btn-soft btn-info shrink-0 gap-1.5"
                onClick={() =>
                  navigate(
                    `/collections/folders/${encodeFolderKey({
                      repositoryId: child.repository_id ?? identity.repositoryId,
                      folderPath: child.folder_path ?? "",
                    })}`,
                  )
                }
              >
                {child.cover_asset_id ? (
                  <span className="avatar">
                    <span className="size-5 rounded-full">
                      <img
                        src={assetUrls.getThumbnailUrl(child.cover_asset_id, "small")}
                        alt=""
                        className="rounded-full object-cover"
                      />
                    </span>
                  </span>
                ) : (
                  <FolderIcon className="h-3.5 w-3.5" />
                )}
                {child.display_name}
                <span className="opacity-60">({child.asset_count ?? 0})</span>
              </button>
            ))}
          </div>
        )
      }
    />
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

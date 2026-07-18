import { Navigate, useParams } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { useCallback, useMemo, useState } from "react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetBrowser, AssetBrowserScope, type AssetBrowseConstraint } from "@/features/assets";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { findUtilityClassifier, getUtilityClassifierTitle } from "../../model/utilityClassifiers";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import { CreateShareLinkModal, createShareSelectedBulkAction } from "@/features/share";

export default function UtilityClassifierAlbum() {
  const { classifierSlug } = useParams<{ classifierSlug: string }>();
  const { t } = useI18n();
  const classifier = findUtilityClassifier(classifierSlug);

  const constraint: AssetBrowseConstraint = useMemo(
    () => ({
      tag_name: classifier?.tagName,
      tag_source: "zeroshot",
    }),
    [classifier?.tagName],
  );

  const [shareAssetIds, setShareAssetIds] = useState<string[] | null>(null);
  const bulkActions = useCallback(
    (_context: AssetsBulkActionContext): AssetsBulkActionItem[] => [
      createShareSelectedBulkAction(
        t("assets.assetsPageHeader.bulkActions.share.label", "Share"),
        setShareAssetIds,
      ),
    ],
    [t],
  );

  useBreadcrumbs(
    classifier
      ? [
          { label: t("sidebar.home", "Home"), to: "/" },
          { label: t("sidebar.collections", "Collections"), to: "/collections" },
          {
            label: t("collections.sections.utilities", "Utilities"),
            to: "/collections/utilities",
          },
          { label: getUtilityClassifierTitle(t, classifier.slug) },
        ]
      : [],
  );

  if (!classifier) {
    return <Navigate to="/collections" replace />;
  }

  const Icon = classifier.icon;
  const basePath = `/collections/utilities/${classifier.slug}`;

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallback
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <AssetBrowserScope scopeId={`collections:utilities:${classifier.slug}`} basePath={basePath}>
        <WorkerProvider>
          <AssetBrowser
            title={getUtilityClassifierTitle(t, classifier.slug)}
            icon={<Icon className="w-6 h-6 text-primary" />}
            constraint={constraint}
            viewKey={`utility:${classifier.slug}`}
            bulkActions={bulkActions}
          />
        </WorkerProvider>
      </AssetBrowserScope>
      <CreateShareLinkModal
        open={shareAssetIds !== null}
        onClose={() => setShareAssetIds(null)}
        sourceKind="asset_snapshot"
        assetIds={shareAssetIds ?? undefined}
      />
    </ErrorBoundary>
  );
}

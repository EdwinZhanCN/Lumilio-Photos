import { Navigate, useParams } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { useMemo } from "react";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetsProvider } from "@/features/assets";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import {
  findUtilityClassifier,
  getUtilityClassifierTitle,
} from "../utils/utilityClassifiers";
import type { AssetFilter } from "@/features/assets/types/assets.type";

export default function UtilityClassifierAlbum() {
  const { classifierSlug } = useParams<{ classifierSlug: string }>();
  const { t } = useI18n();
  const classifier = findUtilityClassifier(classifierSlug);

  const baseFilter: AssetFilter = useMemo(
    () => ({
      tag_name: classifier?.tagName,
      tag_source: "zeroshot",
    }),
    [classifier?.tagName],
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
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <AssetsProvider
        scopeId={`collections:utilities:${classifier.slug}`}
        syncUrl
        basePath={basePath}
      >
        <WorkerProvider>
          <AssetsGalleryPage
            title={getUtilityClassifierTitle(t, classifier.slug)}
            icon={<Icon className="w-6 h-6 text-primary" />}
            baseFilter={baseFilter}
            viewKey={`utility:${classifier.slug}`}
          />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
}

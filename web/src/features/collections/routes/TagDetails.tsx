import { Navigate, useParams } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { useMemo } from "react";
import { Tag as TagIcon } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetsProvider } from "@/features/assets";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import type { AssetFilter } from "@/features/assets/types/assets.type";
import { decodeTagKey } from "../utils/tagKey";

export default function TagDetails() {
  const { tagKey } = useParams<{ tagKey: string }>();
  const { t } = useI18n();
  const identity = useMemo(() => decodeTagKey(tagKey), [tagKey]);

  const baseFilter: AssetFilter = useMemo(
    () => ({
      tag_name: identity?.tagName,
      tag_source: identity?.source || undefined,
    }),
    [identity?.tagName, identity?.source],
  );

  useBreadcrumbs(
    identity
      ? [
          { label: t("sidebar.home", "Home"), to: "/" },
          { label: t("sidebar.collections", "Collections"), to: "/collections" },
          { label: t("collections.sections.tags", "Tags"), to: "/collections/tags" },
          { label: identity.tagName },
        ]
      : [],
  );

  if (!identity) {
    return <Navigate to="/collections/tags" replace />;
  }

  const basePath = `/collections/tags/${tagKey}`;

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
      <AssetsProvider scopeId={`collections:tags:${tagKey}`} syncUrl basePath={basePath}>
        <WorkerProvider>
          <AssetsGalleryPage
            title={identity.tagName}
            icon={<TagIcon className="w-6 h-6 text-primary" />}
            baseFilter={baseFilter}
            viewKey={`tag:${identity.tagName}:${identity.source}`}
          />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
}

import { useLocation, useSearchParams } from "react-router-dom";
import { useCallback, useState } from "react";
import { AssetBrowserScope } from "../browse/selection/AssetBrowserScope";
import { ErrorBoundary } from "react-error-boundary";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallback from "@/components/ui/ErrorFallback";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { $api } from "@/lib/http-commons/queryClient";
import { AssetBrowser } from "../browse/AssetBrowser";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import { CreateShareLinkModal, createShareSelectedBulkAction } from "@/features/share";

interface AssetsOrigin {
  from?: string;
  fromLabel?: string;
  label?: string;
}

const Assets = () => {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const pin = searchParams.get("pin");
  const isPinMode = Boolean(pin);

  // In pin mode the gallery is a leaf reached from elsewhere (typically the
  // agent board). Show a back-crumb to wherever we came from; fall back to the
  // board when the origin state is missing (e.g. on hard refresh / deep-link).
  const origin = (location.state ?? null) as AssetsOrigin | null;
  const pinMetaQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}",
    { params: { path: { id: pin ?? "" } } },
    { enabled: isPinMode, retry: false, staleTime: 60_000 },
  );
  const pinMeta = pinMetaQuery.data;
  useBreadcrumbs(
    isPinMode
      ? [
          {
            label: origin?.fromLabel ?? t("lumilio.nav.board", "Board"),
            to: origin?.from ?? "/lumilio",
          },
          {
            label: pinMeta?.title ?? origin?.label ?? t("assets.pinTrailLabel", "Selection"),
          },
        ]
      : [],
  );

  // Pin deep-links use an isolated selection scope. Browse state is URL-owned,
  // so the `pin` parameter survives carousel navigation without store syncing.
  const scopeId = isPinMode ? `assets:pin:${pin}` : "assets:main";

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
      <AssetBrowserScope key={scopeId} scopeId={scopeId}>
        <WorkerProvider>
          <AssetBrowser
            pinId={pin ?? undefined}
            bulkActions={bulkActions}
            migrateLegacyState={!isPinMode}
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
};

export default Assets;

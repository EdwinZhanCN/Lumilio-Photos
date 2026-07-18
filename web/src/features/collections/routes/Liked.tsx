import { useCallback, useState } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { useQueryClient } from "@tanstack/react-query";
import { Heart, HeartOff } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import {
  AssetBrowser,
  AssetBrowserScope,
  useAssetActions,
  type AssetBrowseConstraint,
} from "@/features/assets";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import { CreateShareLinkModal, createShareSelectedBulkAction } from "@/features/share";

const HIDDEN_LIKED_BULK_ACTIONS = ["set-liked"] as const;
// Module-level constant so the reference is stable across renders — an
// Keep the page constraint stable so the browse query definition is stable.
const LIKED_CONSTRAINT: AssetBrowseConstraint = { liked: true };

const LikedContent = () => {
  const { t } = useI18n();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    {
      label: t("collections.sections.utilities", "Utilities"),
      to: "/collections/utilities",
    },
    { label: t("collections.utilities.liked.title", "Liked") },
  ]);
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { batchUpdateAssets } = useAssetActions();
  const [shareAssetIds, setShareAssetIds] = useState<string[] | null>(null);

  const invalidateAssetLists = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (!Array.isArray(key)) return false;

        const path = key[1];
        return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
      },
    });
  }, [queryClient]);

  const bulkActions = useCallback(
    (context: AssetsBulkActionContext): AssetsBulkActionItem[] => [
      createShareSelectedBulkAction(
        t("assets.assetsPageHeader.bulkActions.share.label", "Share"),
        setShareAssetIds,
      ),
      {
        id: "unlike-assets",
        label: t("collections.utilities.liked.bulkActions.unlike.label", {
          count: context.affectedAssetCount,
        }),
        icon: <HeartOff size={16} />,
        requiresConfirmation: true,
        confirmationTitle: t("collections.utilities.liked.bulkActions.unlike.confirmTitle"),
        confirmationMessage: t("collections.utilities.liked.bulkActions.unlike.confirmMessage", {
          count: context.affectedAssetCount,
        }),
        onRun: async ({ selectedAssetIds, affectedAssetCount, clearSelection }) => {
          try {
            await batchUpdateAssets(
              selectedAssetIds.map((assetId) => ({
                assetId,
                updates: { liked: false },
              })),
            );
            await invalidateAssetLists();
            clearSelection();
            showMessage(
              "success",
              t("collections.utilities.liked.messages.unlikeSuccess", {
                count: affectedAssetCount,
              }),
            );
          } catch (error) {
            console.error("Failed to unlike selected assets:", error);
            showMessage("error", t("collections.utilities.liked.messages.unlikeError"));
            throw error;
          }
        },
      },
    ],
    [batchUpdateAssets, invalidateAssetLists, showMessage, t],
  );

  return (
    <>
      <AssetBrowserScope scopeId="collections:liked" basePath="/collections/liked">
        <WorkerProvider>
          <AssetBrowser
            title={t("collections.utilities.liked.title")}
            icon={<Heart className="h-6 w-6 text-primary" strokeWidth={1.5} />}
            constraint={LIKED_CONSTRAINT}
            viewKey="collections:liked"
            bulkActions={bulkActions}
            hiddenBulkActions={HIDDEN_LIKED_BULK_ACTIONS}
          />
        </WorkerProvider>
      </AssetBrowserScope>
      <CreateShareLinkModal
        open={shareAssetIds !== null}
        onClose={() => setShareAssetIds(null)}
        sourceKind="asset_snapshot"
        assetIds={shareAssetIds ?? undefined}
      />
    </>
  );
};

const Liked = () => {
  const { t } = useI18n();

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
      <LikedContent />
    </ErrorBoundary>
  );
};

export default Liked;

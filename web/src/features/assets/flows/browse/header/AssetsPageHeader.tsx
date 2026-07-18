import { useMemo } from "react";
import { useI18n } from "@/lib/i18n";
import AssetsPageHeaderActions from "./AssetsPageHeaderActions";
import AssetsPageHeaderModals from "./AssetsPageHeaderModals";
import type { AssetsPageHeaderProps } from "./types";
import { useAssetsPageHeaderBulkActions } from "./useAssetsPageHeaderBulkActions";
import { useAssetsPageHeaderScan } from "./useAssetsPageHeaderScan";
import {
  getConstrainedFilterKeys,
  getConstraintUserFilter,
  normalizeAssetUserFilter,
  stripConstrainedAssetUserFilter,
} from "../../../model/filter";

export default function AssetsPageHeader({
  sortBy,
  onSortByChange,
  filter,
  constraint,
  onFiltersChange,
  title,
  subtitle,
  icon,
  browseItems,
  bulkActions,
  hiddenBulkActions,
  capabilities,
  scopeControlHidden,
}: AssetsPageHeaderProps) {
  const { t } = useI18n();
  const activeSortByLabel = useMemo(
    () =>
      sortBy === "recently_added"
        ? t("assets.assetsPageHeader.sortByOptions.recentlyAdded")
        : t("assets.assetsPageHeader.sortByOptions.dateCaptured"),
    [sortBy, t],
  );
  const tabTitle = useMemo(() => t("assets.all.title"), [t]);
  const inboundDTO = useMemo(
    () =>
      normalizeAssetUserFilter({
        ...filter,
        ...getConstraintUserFilter(constraint),
      }),
    [constraint, filter],
  );
  const lockedFilterFields = useMemo(
    () => Array.from(getConstrainedFilterKeys(constraint)),
    [constraint],
  );
  const handleFiltersChange = (nextFilter: typeof inboundDTO) => {
    onFiltersChange(stripConstrainedAssetUserFilter(nextFilter, constraint));
  };
  const bulk = useAssetsPageHeaderBulkActions({
    browseItems,
    bulkActions,
    hiddenBulkActions,
  });
  const scan = useAssetsPageHeaderScan();

  return (
    <>
      <AssetsPageHeaderActions
        title={title}
        subtitle={subtitle}
        icon={icon}
        tabTitle={tabTitle}
        sortBy={sortBy}
        onSortByChange={onSortByChange}
        activeSortByLabel={activeSortByLabel}
        inboundDTO={inboundDTO}
        handleFiltersChange={handleFiltersChange}
        lockedFilterFields={lockedFilterFields}
        scopeControlHidden={scopeControlHidden}
        showScan={capabilities?.showScan ?? true}
        isScanning={scan.isScanning}
        repositoriesLength={scan.repositoriesLength}
        scopeLabel={scan.scopeLabel}
        handleScanCurrentLibrary={scan.handleScanCurrentLibrary}
        bulk={bulk}
      />
      <AssetsPageHeaderModals bulk={bulk} />
    </>
  );
}

import PageHeader from "@/components/ui/PageHeader";
import { BrowseScopeSelect } from "@/features/repositories";
import { useI18n } from "@/lib/i18n";
import {
  ArrowUpDown,
  Ellipsis,
  FunnelIcon,
  ImageIcon,
  RefreshCcwDot,
  Rocket,
  SquareMousePointer,
} from "lucide-react";
import FilterTool, { type FilterDTO } from "../../page/FilterTool/FilterTool";
import BulkActionMenuItems from "./BulkActionMenuItems";
import { closeActiveDropdown } from "./dropdown";
import type { AssetsPageHeaderProps } from "./types";
import type { AssetsPageHeaderBulkActions } from "./useAssetsPageHeaderBulkActions";

interface AssetsPageHeaderActionsProps {
  title: AssetsPageHeaderProps["title"];
  subtitle: AssetsPageHeaderProps["subtitle"];
  icon: AssetsPageHeaderProps["icon"];
  tabTitle: string;
  sortBy: AssetsPageHeaderProps["sortBy"];
  onSortByChange: AssetsPageHeaderProps["onSortByChange"];
  activeSortByLabel: string;
  inboundDTO: FilterDTO;
  handleFiltersChange: (filters: FilterDTO) => void;
  lockedFilterFields: AssetsPageHeaderProps["lockedFilterFields"];
  scopeControlHidden: AssetsPageHeaderProps["scopeControlHidden"];
  showScan: boolean;
  isScanning: boolean;
  repositoriesLength: number;
  scopeLabel: string;
  handleScanCurrentLibrary: () => Promise<void>;
  bulk: AssetsPageHeaderBulkActions;
}

export default function AssetsPageHeaderActions({
  title,
  subtitle,
  icon,
  tabTitle,
  sortBy,
  onSortByChange,
  activeSortByLabel,
  inboundDTO,
  handleFiltersChange,
  lockedFilterFields,
  scopeControlHidden,
  showScan,
  isScanning,
  repositoriesLength,
  scopeLabel,
  handleScanCurrentLibrary,
  bulk,
}: AssetsPageHeaderActionsProps) {
  const { t } = useI18n();
  const { selection, hasBulkActionItems, handleToggleSelection } = bulk;

  return (
    <PageHeader
      title={title ?? tabTitle}
      subtitle={subtitle}
      icon={icon ?? <ImageIcon className="w-6 h-6 text-primary" />}
      className="sticky top-0 z-40 bg-base-100 border-b border-base-200"
    >
      {selection.enabled && (
        <div className="badge badge-lg badge-neutral hidden gap-2 rounded-full px-3 py-3 text-xs font-medium sm:inline-flex shrink-0">
          {t("assets.assetsPageHeader.selectionMode.selectedCount", {
            count: selection.selectedCount,
            defaultValue: "{{count}} selected",
          })}
        </div>
      )}

      {/*
        FULL MODE ACTIONS
        Visible on larger screens when search is NOT active
      */}
      <div className="items-center gap-2 hidden lg:flex">
        {/* Sort By Dropdown */}
        <div className="dropdown">
          <div tabIndex={0} role="button" className="btn btn-sm btn-soft btn-info rounded-full">
            <FunnelIcon className="size-4" />
            {t("assets.assetsPageHeader.sortBy", {
              sortBy: activeSortByLabel,
            })}
          </div>
          <ul
            tabIndex={0}
            className="dropdown-content menu bg-base-200 rounded-box z-[100] w-40 p-2 shadow-xl"
          >
            <li>
              <button
                onClick={() => onSortByChange("date_captured")}
                className={sortBy === "date_captured" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.sortByOptions.dateCaptured")}
              </button>
            </li>
            <li>
              <button
                onClick={() => onSortByChange("recently_added")}
                className={sortBy === "recently_added" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.sortByOptions.recentlyAdded")}
              </button>
            </li>
          </ul>
        </div>

        {!scopeControlHidden && <BrowseScopeSelect className="max-w-[12rem]" />}

        <FilterTool
          initial={inboundDTO}
          onChange={handleFiltersChange}
          autoApply={true}
          lockedFields={lockedFilterFields}
        />

        {showScan && (
          <button
            type="button"
            className="btn btn-sm btn-soft btn-info gap-2 rounded-full"
            onClick={handleScanCurrentLibrary}
            disabled={isScanning || repositoriesLength === 0}
            title={t("assets.assetsPageHeader.scan.title", {
              scope: scopeLabel,
            })}
          >
            {isScanning ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <RefreshCcwDot className="h-4 w-4" />
            )}
            <span className="hidden xl:inline text-xs font-medium">
              {t("assets.assetsPageHeader.scan.label")}
            </span>
          </button>
        )}

        <button
          type="button"
          className={`btn btn-sm btn-soft btn-info gap-2 rounded-full ${
            selection.enabled ? "btn-active" : ""
          }`}
          onClick={handleToggleSelection}
          title={
            selection.enabled
              ? t("assets.assetsPageHeader.selectionMode.exit")
              : t("assets.assetsPageHeader.selectionMode.enter")
          }
        >
          <SquareMousePointer className="w-4 h-4" />
          <span className="hidden xl:inline text-xs font-medium">
            {t("assets.assetsPageHeader.selectionMode.label", {
              defaultValue: "Select",
            })}
          </span>
        </button>

        {/* Quick Actions Rocket Menu - Only in selection mode */}
        {selection.enabled && hasBulkActionItems && (
          <div className="dropdown dropdown-end">
            <div
              tabIndex={0}
              role="button"
              className={`btn btn-sm btn-soft btn-accent gap-2 rounded-full ${
                selection.selectedCount === 0 ? "btn-disabled opacity-50" : ""
              }`}
              title={t("assets.assetsPageHeader.actions.title")}
            >
              <Rocket className="size-4" />
              <span className="hidden xl:inline">{t("assets.assetsPageHeader.actions.title")}</span>
              <span className="rounded-full bg-base-100/90 px-2.5 py-1 text-[11px] font-semibold text-base-content/70">
                {selection.selectedCount}
              </span>
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu z-[100] mt-2 w-64 rounded-box bg-base-100 p-3 shadow-xl border border-base-200"
            >
              <li className="menu-title px-3 py-2 text-xs uppercase tracking-[0.18em] text-base-content/45">
                {t("assets.assetsPageHeader.actions.applyToItems", {
                  count: selection.selectedCount,
                })}
              </li>
              <BulkActionMenuItems bulk={bulk} includeDivider />
            </ul>
          </div>
        )}
      </div>

      {/*
        COMPACT MODE ACTIONS MENU
        Visible on small screens
      */}
      <div className="dropdown dropdown-end m-0 shrink-0 lg:hidden block">
        <div
          tabIndex={0}
          role="button"
          className="btn btn-sm btn-soft btn-circle btn-info m-0"
          title={t("assets.assetsPageHeader.moreActions")}
        >
          <Ellipsis className="w-4 h-4" />
        </div>
        <ul
          tabIndex={0}
          className="dropdown-content menu bg-base-100 rounded-box z-[100] w-56 p-2 shadow-xl border border-base-200 mt-2"
        >
          <li className="menu-title text-xs text-base-content/45">
            {t("assets.assetsPageHeader.compactMenu.viewSort", {
              defaultValue: "View & Sort",
            })}
          </li>
          <li>
            <details open>
              <summary className="py-2">
                <ArrowUpDown size={16} />
                {t("assets.assetsPageHeader.sortBy", { sortBy: "" })}
              </summary>
              <ul>
                <li>
                  <button
                    onClick={() => {
                      onSortByChange("date_captured");
                      closeActiveDropdown();
                    }}
                    className={sortBy === "date_captured" ? "active" : ""}
                  >
                    {t("assets.assetsPageHeader.sortByOptions.dateCaptured")}
                  </button>
                </li>
                <li>
                  <button
                    onClick={() => {
                      onSortByChange("recently_added");
                      closeActiveDropdown();
                    }}
                    className={sortBy === "recently_added" ? "active" : ""}
                  >
                    {t("assets.assetsPageHeader.sortByOptions.recentlyAdded")}
                  </button>
                </li>
              </ul>
            </details>
          </li>

          <li className="menu-title mt-2 text-xs text-base-content/45">
            {t("assets.assetsPageHeader.compactMenu.actions", {
              defaultValue: "Actions",
            })}
          </li>
          <li>
            <button
              onClick={() => {
                handleToggleSelection();
                closeActiveDropdown();
              }}
              className={selection.enabled ? "text-primary font-medium" : ""}
            >
              <SquareMousePointer size={16} />
              {selection.enabled
                ? t("assets.assetsPageHeader.selectionMode.exit")
                : t("assets.assetsPageHeader.selectionMode.enter")}
            </button>
          </li>
          {showScan && (
            <li>
              <button
                onClick={() => {
                  void handleScanCurrentLibrary();
                  closeActiveDropdown();
                }}
                disabled={isScanning || repositoriesLength === 0}
              >
                {isScanning ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <RefreshCcwDot size={16} />
                )}
                {t("assets.assetsPageHeader.scan.label")}
              </button>
            </li>
          )}

          {selection.enabled && hasBulkActionItems && (
            <>
              <li className="menu-title mt-2 text-xs text-base-content/45">
                <span>
                  {t("assets.assetsPageHeader.compactMenu.selectedItems", {
                    defaultValue: "Selected Items",
                  })}{" "}
                  ({selection.selectedCount})
                </span>
              </li>
              <li>
                <details>
                  <summary className="py-2 text-accent">
                    <Rocket size={16} />
                    {t("assets.assetsPageHeader.actions.title")}
                  </summary>
                  <ul className="w-full">
                    <BulkActionMenuItems bulk={bulk} compact />
                  </ul>
                </details>
              </li>
            </>
          )}
        </ul>
      </div>

      {/* Filter Tool in Compact Mode (placed outside Ellipsis menu because it's a complex component) */}
      <div className="shrink-0 ml-2 lg:hidden block">
        <FilterTool
          initial={inboundDTO}
          onChange={handleFiltersChange}
          autoApply={true}
          lockedFields={lockedFilterFields}
        />
      </div>

      {/*
        Repository scope select in Compact Mode, placed outside the Ellipsis
        dropdown-content: nesting a native <select> inside a CSS
        :focus-within-driven dropdown closes the dropdown the instant the
        native select popup opens (it steals focus), so the click never
        reaches an option.
      */}
      {!scopeControlHidden && repositoriesLength > 0 && (
        <div className="shrink-0 order-first w-full basis-full lg:hidden block">
          <BrowseScopeSelect className="w-full" />
        </div>
      )}
    </PageHeader>
  );
}

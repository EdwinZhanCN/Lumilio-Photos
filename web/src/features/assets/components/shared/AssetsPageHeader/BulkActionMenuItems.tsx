import { Download, FolderPlus, Heart, Star, Tags, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { closeActiveDropdown } from "./dropdown";
import type { AssetsPageHeaderBulkActions } from "./useAssetsPageHeaderBulkActions";

interface BulkActionMenuItemsProps {
  bulk: AssetsPageHeaderBulkActions;
  compact?: boolean;
  includeDivider?: boolean;
}

export default function BulkActionMenuItems({
  bulk,
  compact = false,
  includeDivider = false,
}: BulkActionMenuItemsProps) {
  const { t } = useI18n();
  const {
    visibleCustomBulkActions,
    stackSelectedBulkAction,
    isDefaultActionHidden,
    handleDeleteClick,
    handleDownloadAll,
    handleAddToAlbumClick,
    handleAddTagsClick,
    handleCustomBulkActionClick,
    isRunningCustomAction,
    setConfirmableBulkAction,
  } = bulk;
  const ratingOptions = [
    { rating: 5, label: "★★★★★", valueLabel: "5" },
    { rating: 4, label: "★★★★", valueLabel: "4" },
    { rating: 3, label: "★★★", valueLabel: "3" },
    { rating: 2, label: "★★", valueLabel: "2" },
    { rating: 1, label: "★", valueLabel: "1" },
    {
      rating: 0,
      label: t("assets.assetsPageHeader.actions.unrated", {
        defaultValue: "Unrated",
      }),
      valueLabel: "0",
    },
  ];
  const hasTrailingActions =
    !isDefaultActionHidden("add-tags") ||
    !isDefaultActionHidden("add-to-album") ||
    !isDefaultActionHidden("download") ||
    !isDefaultActionHidden("delete-assets");

  return (
    <>
      {!isDefaultActionHidden("set-rating") && (
        <li>
          <details>
            <summary>
              <Star size={16} />
              {t("assets.assetsPageHeader.actions.setRating", {
                defaultValue: "Set Rating",
              })}
            </summary>
            <ul>
              {ratingOptions.map((option) => (
                <li key={option.rating}>
                  <button
                    type="button"
                    onClick={() => {
                      setConfirmableBulkAction({
                        type: "rating",
                        rating: option.rating,
                      });
                      closeActiveDropdown();
                    }}
                  >
                    <span className="min-w-20">{option.label}</span>
                    <span className="ml-auto opacity-50">{option.valueLabel}</span>
                  </button>
                </li>
              ))}
            </ul>
          </details>
        </li>
      )}
      {!isDefaultActionHidden("set-liked") && (
        <li>
          <details>
            <summary>
              <Heart size={16} />
              {t("assets.assetsPageHeader.actions.likedMenu", {
                defaultValue: "Liked",
              })}
            </summary>
            <ul>
              <li>
                <button
                  type="button"
                  onClick={() => {
                    setConfirmableBulkAction({ type: "liked", liked: true });
                    closeActiveDropdown();
                  }}
                >
                  {t("assets.assetsPageHeader.actions.like", {
                    defaultValue: "Liked",
                  })}
                </button>
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => {
                    setConfirmableBulkAction({ type: "liked", liked: false });
                    closeActiveDropdown();
                  }}
                >
                  {t("assets.assetsPageHeader.actions.unlike", {
                    defaultValue: "Unliked",
                  })}
                </button>
              </li>
            </ul>
          </details>
        </li>
      )}
      {!isDefaultActionHidden("stack-selected") && (
        <li>
          <button
            type="button"
            className="text-info"
            disabled={stackSelectedBulkAction.disabled || isRunningCustomAction}
            onClick={() => {
              handleCustomBulkActionClick(stackSelectedBulkAction);
              closeActiveDropdown();
            }}
          >
            {stackSelectedBulkAction.icon}
            {stackSelectedBulkAction.label}
          </button>
        </li>
      )}
      {visibleCustomBulkActions.map((action) => (
        <li key={action.id}>
          <button
            type="button"
            className={
              action.tone === "danger"
                ? "text-error focus:bg-error/20"
                : action.tone === "info"
                  ? "text-info"
                  : undefined
            }
            disabled={action.disabled || isRunningCustomAction}
            onClick={() => {
              handleCustomBulkActionClick(action);
              closeActiveDropdown();
            }}
          >
            {action.icon}
            {action.label}
          </button>
        </li>
      ))}
      {includeDivider && hasTrailingActions && <div className="divider my-1"></div>}
      {!isDefaultActionHidden("add-tags") && (
        <li>
          <button
            type={compact ? undefined : "button"}
            onClick={() => {
              handleAddTagsClick();
              closeActiveDropdown();
            }}
          >
            <Tags size={16} />
            {t("assets.assetsPageHeader.actions.addTags", {
              defaultValue: "Add tags",
            })}
          </button>
        </li>
      )}
      {!isDefaultActionHidden("add-to-album") && (
        <li>
          <button
            type={compact ? undefined : "button"}
            className={compact ? undefined : "text-info"}
            onClick={() => {
              handleAddToAlbumClick();
              closeActiveDropdown();
            }}
          >
            <FolderPlus size={16} className={compact ? "text-info" : undefined} />
            {t("assets.assetsPageHeader.actions.addToAlbum")}
          </button>
        </li>
      )}
      {!isDefaultActionHidden("download") && (
        <li>
          <button
            type={compact ? undefined : "button"}
            onClick={() => {
              void handleDownloadAll();
              closeActiveDropdown();
            }}
          >
            <Download size={16} />
            {t("assets.assetsPageHeader.actions.downloadAll")}
          </button>
        </li>
      )}
      {!isDefaultActionHidden("delete-assets") && (
        <li>
          <button
            type={compact ? undefined : "button"}
            className={compact ? "text-error focus:bg-error/20" : "text-error"}
            onClick={() => {
              handleDeleteClick();
              closeActiveDropdown();
            }}
          >
            <Trash2 size={16} />
            {t("assets.assetsPageHeader.actions.deleteSelected")}
          </button>
        </li>
      )}
    </>
  );
}

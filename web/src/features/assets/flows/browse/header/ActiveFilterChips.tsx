import { XIcon } from "lucide-react";
import { memo, useMemo } from "react";
import { useI18n } from "@/lib/i18n";
import {
  ASSET_USER_FILTER_KEYS,
  isAssetUserFilterFieldActive,
  normalizeAssetUserFilter,
  type AssetUserFilter,
  type AssetUserFilterKey,
  type MediaComposition,
} from "../../../model/filter";

type TFunction = ReturnType<typeof useI18n>["t"];

const COMPOSITION_LABEL_KEYS: Record<MediaComposition, [string, string]> = {
  contains_raw: ["assets.filterTool.compositionSection.contains_raw", "Contains RAW"],
  jpeg_raw: ["assets.filterTool.compositionSection.jpeg_raw", "JPEG + RAW"],
  raw_unpaired: ["assets.filterTool.compositionSection.raw_unpaired", "Unpaired RAW"],
  no_raw: ["assets.filterTool.compositionSection.no_raw", "No RAW"],
  live_photo: ["assets.filterTool.compositionSection.live_photo", "Live Photo"],
};

function stackLabel(filter: AssetUserFilter, t: TFunction): string {
  const stack = filter.stack;
  if (stack?.membership === "unstacked") {
    return t("assets.filterTool.stackSection.unstacked", "Ungrouped");
  }
  if (stack?.kinds?.length === 1) {
    return stack.kinds[0] === "burst"
      ? t("assets.filterTool.stackSection.burst", "Burst")
      : t("assets.filterTool.stackSection.manual", "Manual stack");
  }
  return t("assets.filterTool.stackSection.stacked", "All stacks");
}

function chipLabel(filter: AssetUserFilter, key: AssetUserFilterKey, t: TFunction): string {
  switch (key) {
    case "type":
      return filter.type === "VIDEO"
        ? t("assets.filterTool.typeSection.video")
        : t("assets.filterTool.typeSection.photo");
    case "media_item": {
      const composition = filter.media_item?.composition;
      const [labelKey, fallback] = COMPOSITION_LABEL_KEYS[composition!];
      return t(labelKey, fallback);
    }
    case "stack":
      return stackLabel(filter, t);
    case "rating":
      return filter.rating === 0
        ? t("assets.filterTool.ratingSection.unrated_title")
        : t("assets.filterTool.ratingSection.rating_n", { n: filter.rating });
    case "liked":
      return filter.liked
        ? t("assets.filterTool.likeSection.liked")
        : t("assets.filterTool.likeSection.unliked");
    case "filename":
      return filter.filename!.value;
    case "date":
      return [filter.date?.from, filter.date?.to].filter(Boolean).join(" – ");
    case "camera_model":
      return filter.camera_model!;
    case "lens":
      return filter.lens!;
    case "tag_names":
      return filter.tag_names!.join(", ");
    case "location":
      return t("assets.filterTool.locationSection.title");
  }
}

interface ActiveFilterChipsProps {
  filter: AssetUserFilter;
  onRemove: (key: AssetUserFilterKey) => void;
}

export const ActiveFilterChips = memo(function ActiveFilterChips({
  filter,
  onRemove,
}: ActiveFilterChipsProps) {
  const { t } = useI18n();
  const normalized = useMemo(() => normalizeAssetUserFilter(filter), [filter]);
  const activeKeys = useMemo(
    () => ASSET_USER_FILTER_KEYS.filter((key) => isAssetUserFilterFieldActive(normalized, key)),
    [normalized],
  );

  if (activeKeys.length === 0) return null;

  return (
    <div className="basis-full flex flex-wrap items-center gap-1 justify-end">
      {activeKeys.map((key) => (
        <button
          key={key}
          type="button"
          className="badge badge-sm badge-info badge-soft gap-1 pr-1"
          onClick={() => onRemove(key)}
          title={t("assets.assetsPageHeader.activeFilters.remove", "Remove filter")}
        >
          <span className="max-w-40 truncate">{chipLabel(normalized, key, t)}</span>
          <XIcon className="size-3 shrink-0" />
        </button>
      ))}
    </div>
  );
});

export default ActiveFilterChips;

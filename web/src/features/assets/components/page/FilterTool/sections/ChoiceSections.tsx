import { memo } from "react";
import { useI18n } from "@/lib/i18n";
import type { MediaTypeFilter } from "../types";
import { SectionShell } from "./SectionShell";

interface ToggleSectionProps {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (value: boolean) => void;
}

interface RawSectionProps extends ToggleSectionProps {
  mode: "include" | "exclude";
  onModeChange: (mode: "include" | "exclude") => void;
}

export const RawSection = memo(function RawSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  mode,
  onModeChange,
}: RawSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.rawSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${mode === "include" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("include")}
        >
          {t("assets.filterTool.rawSection.include")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${mode === "exclude" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("exclude")}
        >
          {t("assets.filterTool.rawSection.exclude")}
        </button>
      </div>
    </SectionShell>
  );
});

interface TypeSectionProps extends ToggleSectionProps {
  value: MediaTypeFilter;
  onValueChange: (value: MediaTypeFilter) => void;
}

export const TypeSection = memo(function TypeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: TypeSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.typeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === "PHOTO" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange("PHOTO")}
        >
          {t("assets.filterTool.typeSection.photo")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === "VIDEO" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange("VIDEO")}
        >
          {t("assets.filterTool.typeSection.video")}
        </button>
      </div>
    </SectionShell>
  );
});

interface RatingSectionProps extends ToggleSectionProps {
  value: number;
  onValueChange: (value: number) => void;
}

export const RatingSection = memo(function RatingSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: RatingSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.ratingSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full flex">
        {[5, 4, 3, 2, 1].map((rating) => (
          <button
            key={rating}
            type="button"
            className={`btn btn-xs join-item flex-1 ${value === rating ? "btn-primary" : "btn-outline"}`}
            disabled={filterDisabled || !enabled}
            onClick={() => onValueChange(rating)}
            title={t("assets.filterTool.ratingSection.rating_n", { n: rating })}
          >
            {rating}
          </button>
        ))}
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === 0 ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(0)}
          title={t("assets.filterTool.ratingSection.unrated_title")}
        >
          {t("assets.filterTool.ratingSection.unrated_short")}
        </button>
      </div>
    </SectionShell>
  );
});

interface LikeSectionProps extends ToggleSectionProps {
  value: boolean;
  onValueChange: (value: boolean) => void;
}

export const LikeSection = memo(function LikeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: LikeSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.likeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(true)}
        >
          {t("assets.filterTool.likeSection.liked")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${!value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(false)}
        >
          {t("assets.filterTool.likeSection.unliked")}
        </button>
      </div>
    </SectionShell>
  );
});

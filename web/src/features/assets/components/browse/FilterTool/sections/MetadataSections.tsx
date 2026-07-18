import { memo, useState } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useI18n } from "@/lib/i18n";
import TagPickerMenu, { type TagPickerItem } from "../../../media/TagPickerMenu";
import { SectionShell } from "./SectionShell";

type TagOption = components["schemas"]["dto.TagDTO"];

interface ToggleSectionProps {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (value: boolean) => void;
}

interface SelectSectionProps extends ToggleSectionProps {
  value: string;
  onValueChange: (value: string) => void;
  items: string[];
  loading: boolean;
}

export const CameraMakeSection = memo(function CameraMakeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
  items,
  loading,
}: SelectSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.cameraMakeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
      >
        <option value="">{t("assets.filterTool.cameraMakeSection.select_placeholder")}</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">
          {t("assets.filterTool.cameraMakeSection.loading_options")}
        </span>
      )}
    </SectionShell>
  );
});

export const LensSection = memo(function LensSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
  items,
  loading,
}: SelectSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.lensSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
      >
        <option value="">{t("assets.filterTool.lensSection.select_placeholder")}</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">
          {t("assets.filterTool.lensSection.loading_options")}
        </span>
      )}
    </SectionShell>
  );
});

interface TagSectionProps extends ToggleSectionProps {
  value: string[];
  onValueChange: (value: string[]) => void;
}

export const TagSection = memo(function TagSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: TagSectionProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const active = enabled && !filterDisabled;

  const tagsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/tags",
    { params: { query: { q: query, limit: 20 } } },
    { enabled: active, staleTime: 30_000 },
  );
  const options: TagOption[] = tagsQuery.data?.tags ?? [];
  const selected = new Set(value);
  const suggestions: TagPickerItem[] = options
    .filter((tag) => tag.tag_name && !selected.has(tag.tag_name))
    .map((tag) => ({ id: tag.tag_id ?? tag.tag_name!, name: tag.tag_name! }));
  const checked: TagPickerItem[] = value.map((name) => ({ id: name, name }));

  return (
    <SectionShell
      title={t("assets.filterTool.tagSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <TagPickerMenu
        query={query}
        onQueryChange={setQuery}
        placeholder={t("assets.filterTool.tagSection.placeholder")}
        loading={tagsQuery.isFetching}
        loadingText={t("assets.filterTool.tagSection.loading")}
        noResultsText={t("assets.filterTool.tagSection.no_results")}
        checked={checked}
        suggestions={active ? suggestions : []}
        disabled={!active}
        onToggleChecked={(item) => onValueChange(value.filter((name) => name !== item.name))}
        onSelectSuggestion={(item) => onValueChange([...value, item.name])}
        className="max-h-52"
      />
    </SectionShell>
  );
});

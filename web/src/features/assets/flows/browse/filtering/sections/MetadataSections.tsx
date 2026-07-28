import { memo, useState } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useI18n } from "@/lib/i18n";
import TagPickerMenu, { type TagPickerItem } from "../../../../components/TagPickerMenu";
import { SectionShell } from "./SectionShell";

type TagOption = components["schemas"]["dto.TagDTO"];

interface SelectSectionProps {
  locked: boolean;
  value: string;
  onValueChange: (value: string) => void;
  items: string[];
  loading: boolean;
}

export const CameraMakeSection = memo(function CameraMakeSection({
  locked,
  value,
  onValueChange,
  items,
  loading,
}: SelectSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.cameraMakeSection.title")}
      active={value !== ""}
      onClear={() => onValueChange("")}
      locked={locked}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={locked || loading}
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
  locked,
  value,
  onValueChange,
  items,
  loading,
}: SelectSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.lensSection.title")}
      active={value !== ""}
      onClear={() => onValueChange("")}
      locked={locked}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={locked || loading}
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

interface TagSectionProps {
  locked: boolean;
  value: string[];
  onValueChange: (value: string[]) => void;
}

export const TagSection = memo(function TagSection({
  locked,
  value,
  onValueChange,
}: TagSectionProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");

  const tagsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/tags",
    { params: { query: { q: query, limit: 20 } } },
    { enabled: !locked, staleTime: 30_000 },
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
      active={value.length > 0}
      onClear={() => onValueChange([])}
      locked={locked}
    >
      <TagPickerMenu
        query={query}
        onQueryChange={setQuery}
        placeholder={t("assets.filterTool.tagSection.placeholder")}
        loading={tagsQuery.isFetching}
        loadingText={t("assets.filterTool.tagSection.loading")}
        noResultsText={t("assets.filterTool.tagSection.no_results")}
        checked={checked}
        suggestions={locked ? [] : suggestions}
        disabled={locked}
        onToggleChecked={(item) => onValueChange(value.filter((name) => name !== item.name))}
        onSelectSuggestion={(item) => onValueChange([...value, item.name])}
        className="max-h-52"
      />
    </SectionShell>
  );
});

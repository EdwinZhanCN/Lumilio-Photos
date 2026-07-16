import { memo } from "react";
import { useI18n } from "@/lib/i18n";
import type { FilenameOperator } from "../types";
import { SectionShell } from "./SectionShell";

interface ToggleSectionProps {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (value: boolean) => void;
}

interface FilenameSectionProps extends ToggleSectionProps {
  operator: FilenameOperator;
  onOperatorChange: (operator: FilenameOperator) => void;
  value: string;
  onValueChange: (value: string) => void;
}

export const FilenameSection = memo(function FilenameSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  operator,
  onOperatorChange,
  value,
  onValueChange,
}: FilenameSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.filenameSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex flex-col gap-2">
        <select
          className="select select-bordered select-xs w-full"
          disabled={filterDisabled || !enabled}
          value={operator}
          onChange={(event) => onOperatorChange(event.target.value as FilenameOperator)}
        >
          <option value="contains">{t("assets.filterTool.filenameSection.contains")}</option>
          <option value="matches">{t("assets.filterTool.filenameSection.matches")}</option>
          <option value="starts_with">{t("assets.filterTool.filenameSection.starts_with")}</option>
          <option value="ends_with">{t("assets.filterTool.filenameSection.ends_with")}</option>
        </select>
        <input
          type="text"
          className="input input-bordered input-xs w-full"
          placeholder={t("assets.filterTool.filenameSection.placeholder")}
          disabled={filterDisabled || !enabled}
          value={value}
          onChange={(event) => onValueChange(event.target.value)}
        />
      </div>
    </SectionShell>
  );
});

interface DateSectionProps extends ToggleSectionProps {
  from: string;
  onFromChange: (value: string) => void;
  to: string;
  onToChange: (value: string) => void;
}

export const DateSection = memo(function DateSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  from,
  onFromChange,
  to,
  onToChange,
}: DateSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell
      title={t("assets.filterTool.dateSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex flex-col gap-2">
        <label className="input input-bordered input-xs w-full flex items-center gap-2">
          <span className="text-xs opacity-70 w-8">{t("assets.filterTool.dateSection.from")}</span>
          <input
            type="date"
            className="grow text-xs"
            value={from}
            disabled={filterDisabled || !enabled}
            onChange={(event) => onFromChange(event.target.value)}
          />
        </label>
        <label className="input input-bordered input-xs w-full flex items-center gap-2">
          <span className="text-xs opacity-70 w-8">{t("assets.filterTool.dateSection.to")}</span>
          <input
            type="date"
            className="grow text-xs"
            value={to}
            disabled={filterDisabled || !enabled}
            onChange={(event) => onToChange(event.target.value)}
          />
        </label>
      </div>
    </SectionShell>
  );
});

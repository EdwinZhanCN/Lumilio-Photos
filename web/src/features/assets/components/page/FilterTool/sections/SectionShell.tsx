import { memo, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";

interface SectionShellProps {
  title: string;
  enabled: boolean;
  onToggle: (value: boolean) => void;
  disabled?: boolean;
  children: ReactNode;
}

export const SectionShell = memo(function SectionShell({
  title,
  enabled,
  onToggle,
  disabled,
  children,
}: SectionShellProps) {
  const { t } = useI18n();

  return (
    <div className="form-control mb-3">
      <div className="flex items-center justify-between">
        <span className="label-text font-medium">{title}</span>
        <label className="label cursor-pointer p-0 gap-2">
          <span className="label-text">{t("assets.filterTool.sectionShell.enable")}</span>
          <input
            type="checkbox"
            className="toggle toggle-primary toggle-sm"
            disabled={disabled}
            checked={enabled}
            onChange={(event) => onToggle(event.target.checked)}
          />
        </label>
      </div>
      <div className="mt-2">{children}</div>
    </div>
  );
});

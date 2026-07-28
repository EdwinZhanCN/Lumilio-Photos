import { LockIcon } from "lucide-react";
import { memo, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";

interface SectionShellProps {
  title: string;
  /**
   * A section is active purely because it holds a value; there is no enable
   * switch. Sections whose control clears itself (a choice row's reset, a
   * select's "All" option) omit `onClear` and get no header button.
   */
  active?: boolean;
  onClear?: () => void;
  locked?: boolean;
  children: ReactNode;
}

export const SectionShell = memo(function SectionShell({
  title,
  active,
  onClear,
  locked,
  children,
}: SectionShellProps) {
  const { t } = useI18n();

  return (
    <div className="form-control mb-3">
      <div className="flex items-center justify-between min-h-6">
        <span className="label-text font-medium flex items-center gap-1">
          {title}
          {locked && (
            <LockIcon
              className="w-3 h-3 opacity-60"
              aria-label={t("assets.filterTool.sectionShell.locked", "Locked by this view")}
            />
          )}
        </span>
        {onClear && active && !locked && (
          <button type="button" className="btn btn-ghost btn-xs" onClick={onClear}>
            {t("assets.filterTool.sectionShell.clear", "Clear")}
          </button>
        )}
      </div>
      <div className="mt-2">{children}</div>
    </div>
  );
});

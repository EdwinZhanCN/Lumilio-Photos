import React from "react";
import { ChevronRight, type LucideIcon } from "lucide-react";

type SectionHeaderProps = {
  icon?: LucideIcon;
  title: string;
  open: boolean;
  modified?: boolean;
  onToggle: () => void;
};

/** Collapsible group header local to the Develop panel. */
export function SectionHeader({
  icon: Icon,
  title,
  open,
  modified = false,
  onToggle,
}: SectionHeaderProps): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      className="group flex w-full items-center gap-2 px-1 py-1.5 text-left"
    >
      <ChevronRight
        size={14}
        className={`text-base-content/50 transition-transform duration-200 ${
          open ? "rotate-90" : ""
        }`}
      />
      {Icon && <Icon size={14} className="text-base-content/60" />}
      <span className="text-xs font-semibold uppercase tracking-wider text-base-content/70">
        {title}
      </span>
      {modified && <span className="badge badge-primary badge-xs ml-auto" aria-label="modified" />}
    </button>
  );
}

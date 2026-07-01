/**
 * renew design language "B" — inset grouped lists (macOS System Settings).
 *
 * A settings page is a stack of SettingsGroup containers. Each group is a
 * soft, borderless, rounded surface holding hairline-divided rows. Rows
 * lead with a vivid colored icon chip and trail with their control
 * (switch, popup value, chevron). Changes apply instantly — no per-card
 * headers, no state badges, no save bars. Calm and dense.
 *
 *   SettingsGroup   the rounded surface + optional muted section label
 *   SettingsRow     one compact icon + label + trailing-control row
 *   SettingsBlock   a full-width row for tall content (grids, sliders)
 */
import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";

interface SettingsGroupProps {
  /** Section title — sits in the left column on desktop, above on mobile. */
  title?: string;
  description?: string;
  children: ReactNode;
  className?: string;
}

/**
 * Responsive section: on mobile it stacks (title above the inset group);
 * on desktop (lg+) it splits into a Vercel-style two-column layout with the
 * title + description on the left and the grouped controls on the right, so
 * wide screens don't read as empty. All groups share the same left-column
 * width, keeping every control container aligned down the page.
 */
export function SettingsGroup({
  title,
  description,
  children,
  className = "",
}: SettingsGroupProps) {
  return (
    <section
      className={`grid gap-2.5 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-10 ${className}`}
    >
      {(title || description) && (
        <div className="px-1 lg:pt-1">
          {title && <div className="text-sm font-semibold">{title}</div>}
          {description && (
            <p className="mt-1 text-xs leading-relaxed text-base-content/55">{description}</p>
          )}
        </div>
      )}
      <div className="divide-y divide-base-300/50 rounded-2xl bg-base-200/50">{children}</div>
    </section>
  );
}

interface SettingsRowProps {
  /** Leading glyph, rendered inside a colored chip. */
  icon?: ReactNode;
  /** Tailwind classes for the chip background/text, e.g. "bg-info text-info-content". */
  iconColor?: string;
  label: ReactNode;
  description?: ReactNode;
  /** Trailing interactive control (toggle, select, …). */
  control?: ReactNode;
  /** Trailing read/navigable value text. */
  value?: ReactNode;
  /** Show a trailing chevron (navigable row). */
  chevron?: boolean;
  /** Makes the whole row a button. */
  onClick?: () => void;
  /** Disable the row button (dimmed, non-interactive). */
  disabled?: boolean;
  /** Selected state for option rows (renders a trailing check). */
  selected?: boolean;
  htmlFor?: string;
  align?: "center" | "start";
  className?: string;
}

export function SettingsRow({
  icon,
  iconColor = "bg-primary text-primary-content",
  label,
  description,
  control,
  value,
  chevron,
  onClick,
  disabled,
  selected,
  htmlFor,
  align = "center",
  className = "",
}: SettingsRowProps) {
  const alignClass = align === "start" ? "items-start" : "items-center";
  const Label = htmlFor ? "label" : "div";

  const inner = (
    <>
      {icon && (
        <span
          className={`flex size-7 shrink-0 items-center justify-center rounded-lg ${iconColor} ${
            align === "start" ? "mt-0.5" : ""
          }`}
        >
          {icon}
        </span>
      )}
      <div className="min-w-0 flex-1">
        <Label {...(htmlFor ? { htmlFor } : {})} className="block text-sm font-medium">
          {label}
        </Label>
        {description && <div className="mt-0.5 text-xs text-base-content/55">{description}</div>}
      </div>
      <div className="flex shrink-0 items-center gap-2 text-sm text-base-content/60">
        {value}
        {control}
        {selected && (
          <span className="text-primary">
            <CheckGlyph />
          </span>
        )}
        {chevron && <ChevronRight className="size-4 text-base-content/30" />}
      </div>
    </>
  );

  const base = `flex gap-3 px-4 py-3 ${alignClass} ${className}`;

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        aria-pressed={selected}
        className={`${base} w-full text-left transition-colors hover:bg-base-300/30 disabled:pointer-events-none disabled:opacity-50`}
      >
        {inner}
      </button>
    );
  }

  return <div className={base}>{inner}</div>;
}

interface SettingsBlockProps {
  children: ReactNode;
  className?: string;
}

export function SettingsBlock({ children, className = "" }: SettingsBlockProps) {
  return <div className={`px-4 py-3.5 ${className}`}>{children}</div>;
}

function CheckGlyph() {
  return (
    <svg viewBox="0 0 20 20" className="size-4" fill="none" aria-hidden="true">
      <path
        d="M4 10.5l4 4 8-9"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

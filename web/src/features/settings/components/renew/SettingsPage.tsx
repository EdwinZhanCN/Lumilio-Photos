/**
 * renew/ — a clean, self-consistent settings UI kit.
 *
 * Three layers, each owning one job, with a strictly monotonic type scale:
 *
 *   L1  SettingsPage   icon + title + description   text-lg  font-semibold
 *   L2  SettingsCard   bordered section + state     text-base font-semibold
 *   L3  SettingField   one labelled control row      text-sm  font-medium
 *
 * Tabs compose these primitives and never hand-roll header/section/row
 * markup, so spacing, typography, and state affordances stay identical
 * across the whole settings surface.
 */
import type { ReactNode } from "react";

interface SettingsPageProps {
  icon: ReactNode;
  title: string;
  description?: string;
  /** Optional page-level actions, right-aligned in the header. */
  actions?: ReactNode;
  children: ReactNode;
}

export function SettingsPage({ icon, title, description, actions, children }: SettingsPageProps) {
  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          {icon}
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-semibold">{title}</h2>
          {description && <p className="mt-0.5 text-sm text-base-content/60">{description}</p>}
        </div>
        {actions && <div className="shrink-0">{actions}</div>}
      </div>

      <div className="space-y-4">{children}</div>
    </div>
  );
}

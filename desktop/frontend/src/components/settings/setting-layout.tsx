import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function PageHeading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div className="min-w-0">
        {eyebrow ? <p className="page-eyebrow">{eyebrow}</p> : null}
        <h1>{title}</h1>
        <p className="page-description">{description}</p>
      </div>
      {action ? <div className="page-heading-action">{action}</div> : null}
    </div>
  );
}

export function SettingsSection({
  title,
  description,
  children,
  className,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("settings-section", className)}>
      <div className="section-heading">
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
      </div>
      <div className="section-rows">{children}</div>
    </section>
  );
}

export function SettingRow({
  title,
  description,
  children,
  stacked = false,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  stacked?: boolean;
}) {
  return (
    <div className={cn("setting-row", stacked && "setting-row-stacked")}>
      <div className="setting-copy">
        <h3>{title}</h3>
        {description ? <p>{description}</p> : null}
      </div>
      <div className="setting-control">{children}</div>
    </div>
  );
}

export function InlineNotice({
  tone = "neutral",
  title,
  children,
}: {
  tone?: "neutral" | "warning" | "danger" | "success";
  title: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("inline-notice", `inline-notice-${tone}`)} role={tone === "danger" ? "alert" : undefined}>
      <strong>{title}</strong>
      <span>{children}</span>
    </div>
  );
}

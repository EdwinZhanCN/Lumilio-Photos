import React, { type ReactNode } from "react";
import { ArrowRight, CircleAlert, Lock, type LucideIcon } from "lucide-react";
import { AuthShell } from "./Shell.tsx";
import { Btn } from "./Buttons.tsx";

export const InlineError: React.FC<{ children: ReactNode }> = ({ children }) => (
  <div className="flex items-start gap-2 rounded-xl bg-error/10 px-3.5 py-2.5 text-sm text-error">
    <CircleAlert size={15} className="mt-0.5 shrink-0" />
    <span>{children}</span>
  </div>
);

export const SuccessCard: React.FC<{
  icon: LucideIcon;
  title: string;
  subtitle: string;
  ctaLabel: string;
  onCta: () => void;
  badge?: string;
  appName?: string;
}> = ({
  icon: Icon,
  title,
  subtitle,
  ctaLabel,
  onCta,
  badge = "Two-factor authentication active",
  appName,
}) => (
  <AuthShell appName={appName}>
    <div className="flex flex-col items-center gap-5 py-3 text-center">
      <div className="success-ring grid h-20 w-20 place-items-center rounded-full bg-success/12 text-success">
        <Icon size={38} />
      </div>
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-base-content">{title}</h1>
        <p className="mx-auto mt-2 max-w-xs text-sm leading-relaxed text-base-content/55">
          {subtitle}
        </p>
      </div>
      {badge && (
        <div className="flex items-center gap-2 rounded-full bg-base-200 px-3.5 py-1.5 text-xs font-medium text-base-content/60">
          <Lock size={13} /> {badge}
        </div>
      )}
      <Btn variant="neutral" icon={ArrowRight} className="w-full" onClick={onCta}>
        {ctaLabel}
      </Btn>
    </div>
  </AuthShell>
);

import React, { type ReactNode } from "react";
import { type LucideIcon } from "lucide-react";
import { cx } from "./classNames.ts";

export const Brand: React.FC<{
  appName?: string;
  size?: number;
  withWord?: boolean;
  className?: string;
}> = ({ appName = "Lumilio Photos", size = 36, withWord = true, className }) => (
  <div className={cx("flex items-center gap-2.5", className)}>
    <img
      src="/logo.png"
      alt={`${appName} logo`}
      className="shrink-0 object-contain"
      style={{ width: size, height: size }}
    />
    {withWord && (
      <span className="text-[1.35rem] font-semibold tracking-tight text-base-content">
        {appName}
      </span>
    )}
  </div>
);

export const AuthShell: React.FC<{
  children: ReactNode;
  width?: number;
  appName?: string;
}> = ({ children, width = 440, appName }) => (
  <div className="screen-in w-full" style={{ maxWidth: width }}>
    <div className="mb-7 flex justify-center">
      <Brand appName={appName} />
    </div>
    <div className="card border border-base-200 bg-base-100 shadow-[0_1px_2px_rgba(0,0,0,0.04),0_12px_32px_-12px_rgba(0,0,0,0.12)]">
      <div className="card-body gap-5 p-7 sm:p-8">{children}</div>
    </div>
  </div>
);

export type HeadTone = "neutral" | "primary" | "success" | "warning";

const HEAD_TONES: Record<HeadTone, string> = {
  neutral: "bg-base-200 text-base-content",
  primary: "bg-primary/10 text-primary",
  success: "bg-success/15 text-success",
  warning: "bg-warning/15 text-warning",
};

export const CardHead: React.FC<{
  icon?: LucideIcon;
  title: ReactNode;
  sub?: ReactNode;
  tone?: HeadTone;
}> = ({ icon: Icon, title, sub, tone = "neutral" }) => (
  <div className="flex flex-col gap-3">
    {Icon && (
      <div className={cx("grid h-11 w-11 place-items-center rounded-xl", HEAD_TONES[tone])}>
        <Icon size={22} />
      </div>
    )}
    <div>
      <h1 className="text-[1.4rem] font-semibold leading-tight tracking-tight text-base-content">
        {title}
      </h1>
      {sub && <p className="mt-1.5 text-sm leading-relaxed text-base-content/55">{sub}</p>}
    </div>
  </div>
);

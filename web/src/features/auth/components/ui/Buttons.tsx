import React, { useState } from "react";
import { Check, Copy, type LucideIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { cx } from "./classNames.ts";

export type BtnVariant = "primary" | "neutral" | "outline" | "ghost";

const BTN_VARIANTS: Record<BtnVariant, string> = {
  primary: "btn-primary",
  neutral: "btn-neutral",
  outline:
    "btn-outline border-base-300 hover:border-base-content/30 hover:bg-base-200 text-base-content",
  ghost: "btn-ghost text-base-content/70",
};

type BtnProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: BtnVariant;
  loading?: boolean;
  icon?: LucideIcon;
};

export const Btn: React.FC<BtnProps> = ({
  children,
  variant = "primary",
  loading,
  icon: Icon,
  className,
  disabled,
  ...props
}) => (
  <button
    className={cx(
      "btn h-12 min-h-12 gap-2 rounded-xl text-[0.95rem] font-medium normal-case",
      BTN_VARIANTS[variant],
      className,
    )}
    disabled={loading || disabled}
    {...props}
  >
    {loading ? <span className="loading loading-spinner loading-sm" /> : Icon && <Icon size={18} />}
    {children}
  </button>
);

export const CopyButton: React.FC<{
  text: string;
  label?: string;
  copiedLabel?: string;
  variant?: "chip" | "block";
}> = ({ text, label, copiedLabel, variant = "chip" }) => {
  const { t } = useI18n();
  const resolvedLabel = label ?? t("common.copy", { defaultValue: "Copy" });
  const resolvedCopiedLabel = copiedLabel ?? t("common.copied", { defaultValue: "Copied" });
  const [done, setDone] = useState(false);
  const copy = () => {
    void navigator.clipboard?.writeText(text).catch(() => undefined);
    setDone(true);
    window.setTimeout(() => setDone(false), 1600);
  };
  if (variant === "block") {
    return (
      <button
        type="button"
        onClick={copy}
        className="btn btn-outline h-11 min-h-11 w-full gap-2 rounded-xl border-base-300 font-medium normal-case text-base-content hover:bg-base-200"
      >
        {done ? <Check size={16} /> : <Copy size={16} />}{" "}
        {done ? resolvedCopiedLabel : resolvedLabel}
      </button>
    );
  }
  return (
    <button
      type="button"
      onClick={copy}
      className="btn btn-ghost btn-sm gap-1.5 rounded-lg text-base-content/60 hover:text-base-content"
    >
      {done ? <Check size={14} /> : <Copy size={14} />} {done ? resolvedCopiedLabel : resolvedLabel}
    </button>
  );
};

import React from "react";
import { Check, type LucideIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { cx } from "./classNames.ts";

export const FlowSteps: React.FC<{ steps: string[]; current: number }> = ({ steps, current }) => (
  <ul className="mb-1 flex items-center gap-1.5 text-xs font-medium">
    {steps.map((s, i) => {
      const done = i < current;
      const active = i === current;
      return (
        <li key={s} className="flex items-center gap-1.5">
          <span
            className={cx(
              "grid h-5 w-5 place-items-center rounded-full text-[0.65rem]",
              done
                ? "bg-success text-success-content"
                : active
                  ? "bg-neutral text-neutral-content"
                  : "bg-base-200 text-base-content/40",
            )}
          >
            {done ? <Check size={11} /> : i + 1}
          </span>
          <span className={active ? "text-base-content" : "text-base-content/40"}>{s}</span>
          {i < steps.length - 1 && <span className="mx-0.5 h-px w-4 bg-base-300" />}
        </li>
      );
    })}
  </ul>
);

export type StepperStep = { key: string; label: string; icon: LucideIcon };

export const Stepper: React.FC<{ steps: StepperStep[]; current: number }> = ({
  steps,
  current,
}) => {
  const { t } = useI18n();
  return (
    <ol className="flex flex-col gap-1">
      {steps.map((s, i) => {
        const done = i < current;
        const active = i === current;
        const Icon = s.icon;
        return (
          <li key={s.key} className="flex items-center gap-3 rounded-lg px-2.5 py-2">
            <span
              className={cx(
                "grid h-8 w-8 shrink-0 place-items-center rounded-lg transition-colors",
                done
                  ? "bg-success/15 text-success"
                  : active
                    ? "bg-neutral text-neutral-content"
                    : "bg-base-200 text-base-content/35",
              )}
            >
              {done ? <Check size={16} /> : <Icon size={16} />}
            </span>
            <div className="leading-tight">
              <p
                className={cx(
                  "text-sm font-medium",
                  active
                    ? "text-base-content"
                    : done
                      ? "text-base-content/70"
                      : "text-base-content/40",
                )}
              >
                {s.label}
              </p>
              <p className="text-[0.7rem] text-base-content/35">
                {t("auth.bootstrap.stepNum", {
                  defaultValue: "Step {{n}}",
                  n: i + 1,
                })}
              </p>
            </div>
          </li>
        );
      })}
    </ol>
  );
};

import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { stateLabel, statusTone, type StatusTone } from "../../model/capabilityVocabulary";

const STATE_STYLE: Record<StatusTone, { text: string; dot: string }> = {
  ok: { text: "text-success", dot: "status-success" },
  danger: { text: "text-error", dot: "status-error" },
  warning: { text: "text-warning", dot: "status-warning" },
  neutral: { text: "text-base-content/60", dot: "status-neutral" },
};

export function InspectorState({ state }: { state?: string }) {
  const { t } = useI18n();
  const tone = statusTone(state === "active" ? "ready" : state);
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-2 text-xs font-normal ${STATE_STYLE[tone].text}`}
    >
      <span className={`status status-sm ${STATE_STYLE[tone].dot}`} aria-hidden />
      {stateLabel(t, state)}
    </span>
  );
}

export function InspectorFields({ fields }: { fields: [string, ReactNode][] }) {
  return (
    <dl className="space-y-3 text-sm">
      {fields.map(([label, value]) => (
        <div key={label} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <dt className="text-base-content/60">{label}</dt>
          <dd className="ml-auto min-w-0 max-w-full break-words text-right tabular-nums">
            {value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

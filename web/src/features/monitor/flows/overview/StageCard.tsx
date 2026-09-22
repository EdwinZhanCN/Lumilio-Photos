import { useId } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import type { ProcessingStage } from "../../api/useProcessing";
import {
  STATUS_BADGE,
  cardLine,
  stageLabel,
  statusLabel,
  unitLabel,
} from "../../model/stageVocabulary";

/** One processing stage. Every card has the same shape in its own unit. */
export function StageCard({
  stage,
  selected,
  onSelect,
}: {
  stage: ProcessingStage;
  selected: boolean;
  onSelect: () => void;
}) {
  const { t, i18n } = useI18n();
  const id = useId();
  const status = stage.status ?? "idle";
  const remaining = stage.remaining ?? 0;
  const failed = (stage.failed ?? 0) > 0;
  const formatter = new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language);
  return (
    <button
      type="button"
      aria-pressed={selected}
      aria-labelledby={`${id}-name`}
      onClick={onSelect}
      className={`card flex min-h-32 w-full flex-col gap-3 bg-base-100 p-4 text-left transition-colors ${
        selected
          ? "border-2 border-primary"
          : failed
            ? "border-2 border-warning/70"
            : "border border-base-300 hover:border-base-content/30"
      }`}
    >
      <span className="flex w-full items-center gap-2">
        <span id={`${id}-name`} className="flex-1 text-sm font-semibold">
          {stageLabel(t, stage.id)}
        </span>
        <span className={`badge badge-sm ${STATUS_BADGE[status]}`}>{statusLabel(t, status)}</span>
      </span>
      <span className="flex items-baseline gap-1.5">
        <span className="text-3xl font-semibold tabular-nums leading-none">
          {formatter.format(remaining)}
        </span>
        <span className="text-sm text-base-content/60">{unitLabel(t, stage.unit, remaining)}</span>
      </span>
      <span
        className={`text-sm ${failed ? "font-medium text-warning-content" : "text-base-content/60"}`}
      >
        {cardLine(t, stage)}
      </span>
    </button>
  );
}

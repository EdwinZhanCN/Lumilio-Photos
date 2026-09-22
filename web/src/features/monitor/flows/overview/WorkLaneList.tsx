import { useId } from "react";
import { ChevronRight, CircleAlert } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";

/** One non-file kind of Catalog work, reported as exact counts only. */
export interface WorkLane {
  key: string;
  icon: LucideIcon;
  label: string;
  description: string;
  pending: number | undefined;
  failed?: number;
  /** In-app destination that owns this work, when one exists. */
  to?: string;
  linkLabel?: string;
}

type LaneStatus = "unknown" | "attention" | "working" | "clear";

function laneStatus(lane: WorkLane): LaneStatus {
  if (lane.pending == null) return "unknown";
  if ((lane.failed ?? 0) > 0) return "attention";
  return lane.pending > 0 ? "working" : "clear";
}

const STATUS_BADGE: Record<LaneStatus, string> = {
  unknown: "badge-ghost",
  attention: "badge-warning",
  working: "badge-info",
  clear: "badge-success badge-soft",
};

/**
 * The list half of the Processing tab: every kind of work that is not a file
 * in the tray. Rows never animate; they state what the work is, how much is
 * left, what needs attention, and where it is owned.
 */
export function WorkLaneList({ lanes }: { lanes: WorkLane[] }) {
  const { t, i18n } = useI18n();
  const id = useId();
  const formatter = new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language);
  const statusLabel: Record<LaneStatus, string> = {
    unknown: t("monitor.processing.noData", "No data"),
    attention: t("monitor.processing.status.attention", "Needs attention"),
    working: t("monitor.processing.status.working", "In progress"),
    clear: t("monitor.processing.status.clear", "Clear"),
  };

  return (
    <section aria-labelledby={`${id}-title`} className="space-y-3">
      <h2 id={`${id}-title`} className="text-base font-semibold">
        {t("monitor.processing.otherWork", "Other work")}
      </h2>
      <ul className="list rounded-box bg-base-100">
        {lanes.map((lane) => {
          const Icon = lane.icon;
          const status = laneStatus(lane);
          const titleId = `${id}-${lane.key}`;
          return (
            <li
              key={lane.key}
              aria-labelledby={titleId}
              className="list-row grid-cols-[auto_minmax(0,1fr)_auto] items-center"
            >
              <div className="flex size-10 shrink-0 items-center justify-center rounded-box bg-primary/10 text-primary">
                <Icon className="size-5" aria-hidden />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 id={titleId} className="text-sm font-semibold">
                    {lane.label}
                  </h3>
                  <span className={`badge badge-sm ${STATUS_BADGE[status]}`}>
                    {statusLabel[status]}
                  </span>
                </div>
                <p className="mt-1 text-xs text-base-content/60">{lane.description}</p>
                {(lane.failed ?? 0) > 0 && (
                  <p className="mt-1 flex items-center gap-1.5 text-xs text-warning">
                    <CircleAlert className="size-3.5 shrink-0" aria-hidden />
                    {t("monitor.processing.attentionCount", "{{count}} needing attention", {
                      count: lane.failed,
                    })}
                  </p>
                )}
                {lane.to && (
                  <Link
                    to={lane.to}
                    className="link link-hover mt-1 inline-flex items-center gap-0.5 text-xs text-primary"
                  >
                    {lane.linkLabel}
                    <ChevronRight className="size-3.5" aria-hidden />
                  </Link>
                )}
              </div>
              <span
                className={
                  lane.pending == null
                    ? "text-sm text-base-content/60"
                    : "text-2xl font-semibold tabular-nums"
                }
              >
                {lane.pending == null ? "—" : formatter.format(lane.pending)}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

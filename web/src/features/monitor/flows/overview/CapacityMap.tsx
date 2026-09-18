import { useState } from "react";
import type { StorageDiagnostic } from "@/features/repositories";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import { capacityMap } from "../../model/capacityMap";
import "./monitor.css";

export function CapacityMap({ item }: { item: StorageDiagnostic }) {
  const { t } = useI18n();
  const [selected, setSelected] = useState("used");
  const map = capacityMap(item);
  if (!map)
    return (
      <div className="flex min-h-64 items-center justify-center bg-base-200/50 text-sm text-base-content/60">
        {t("monitor.storage.capacityUnknown", "Capacity unavailable")}
      </div>
    );
  const usedWidth = (map.used / map.total) * 100;
  const writableHeight = map.available ? (map.writable / map.available) * 100 : 0;
  const areas = [
    {
      key: "used",
      label: t("monitor.storage.statUsed", "Used"),
      value: map.used,
      tone: "bg-primary text-primary-content",
      style: { left: "0%", top: "0%", width: `${usedWidth}%`, height: "100%" },
    },
    {
      key: "writable",
      label: t("monitor.storage.uploadBudget", "Available for new writes"),
      value: map.writable,
      tone: "bg-base-200",
      style: {
        left: `${usedWidth}%`,
        top: "0%",
        width: `${100 - usedWidth}%`,
        height: `${writableHeight}%`,
      },
    },
    {
      key: "reserve",
      label: t("monitor.storage.safetyReserve", "Safety reserve"),
      value: map.reserve,
      tone: "monitor-reserve",
      style: {
        left: `${usedWidth}%`,
        top: `${writableHeight}%`,
        width: `${100 - usedWidth}%`,
        height: `${100 - writableHeight}%`,
      },
    },
  ];
  return (
    <div className="space-y-4">
      <div
        className="monitor-capacity-map"
        role="group"
        aria-label={t("monitor.storage.capacityHeading", "Capacity")}
      >
        {areas
          .filter((area) => area.value > 0)
          .map((area) => (
            <button
              key={area.key}
              type="button"
              className={`monitor-capacity-area ${area.tone}`}
              style={area.style}
              data-selected={selected === area.key}
              aria-pressed={selected === area.key}
              aria-label={`${area.label}: ${formatBytes(area.value)}`}
              onClick={() => setSelected(area.key)}
            >
              <span className="monitor-capacity-label" aria-hidden>
                <span>{area.label}</span>
                <strong className="text-xl font-medium tabular-nums">
                  {formatBytes(area.value)}
                </strong>
              </span>
            </button>
          ))}
      </div>
      <div
        className="flex flex-wrap gap-x-6 gap-y-3"
        aria-label={t("monitor.storage.mapLegend", "Capacity legend")}
      >
        {areas.map((area) => (
          <button
            key={area.key}
            type="button"
            className="flex items-center gap-2 text-left text-xs"
            aria-pressed={selected === area.key}
            onClick={() => setSelected(area.key)}
          >
            <span className={`size-3 shrink-0 ${area.tone}`} aria-hidden />
            <span>
              {area.label}{" "}
              <strong className="font-medium tabular-nums">{formatBytes(area.value)}</strong>
            </span>
          </button>
        ))}
      </div>
      {/*
        所选区域的数值已由图例按钮和色块本身承担，这里只保留该区域独有的含义。
        小色块会把标签移出几何区域，所以每句话都必须能脱离标签独立读懂。
      */}
      <div
        className="border-t border-base-300 pt-3 text-xs text-base-content/60"
        aria-live="polite"
      >
        {selected === "reserve"
          ? t(
              "monitor.storage.reserveTarget",
              "Reserve target {{target}}; the map shows only space currently retained.",
              { target: formatBytes(item.safety_margin_bytes ?? 0) },
            )
          : selected === "used"
            ? t(
                "monitor.storage.usedMeaning",
                "Total minus filesystem-writable space; this is not strictly physical occupancy.",
              )
            : t(
                "monitor.storage.budgetMeaning",
                "The Server reports this write budget after keeping its safety reserve.",
              )}
      </div>
      <p className="text-xs text-base-content/60 tabular-nums">
        {t("monitor.storage.totalCapacity", "Total {{total}}", { total: formatBytes(map.total) })} ·{" "}
        {t("monitor.storage.statAvailable", "Filesystem writable")}:{" "}
        {formatBytes(item.available_bytes ?? 0)}
      </p>
    </div>
  );
}

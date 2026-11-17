import { useState } from "react";
import { Stethoscope } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { MonitorFrame } from "./MonitorFrame";
import { StageCard } from "./StageCard";
import { OverviewPanel, StageDetailPanel } from "./StagePanel";
import { DiagnosticsDialog } from "./DiagnosticsDialog";
import { useProcessingSummary, type ProcessingStage } from "../../api/useProcessing";

/**
 * Processing: a grid of identical stage cards and one panel. The panel is the
 * overview until a card is selected; the selection lives in the `stage` URL
 * parameter so a detail view is linkable.
 */
export function ProcessingMonitor() {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false);
  const query = useProcessingSummary();
  const summary = query.data;
  const stages = summary?.stages ?? [];
  const selectedId = searchParams.get("stage");
  const selected = stages.find((stage) => stage.id === selectedId);

  const select = (id: ProcessingStage["id"] | null) => {
    const params = new URLSearchParams(searchParams);
    if (id && id !== selectedId) params.set("stage", id);
    else params.delete("stage");
    setSearchParams(params, { replace: true });
  };

  const group = (name: "media" | "catalog", title: string, columns: string) => {
    const cards = stages.filter((stage) => stage.group === name);
    if (cards.length === 0) return null;
    return (
      <section aria-label={title} className="flex flex-col gap-2">
        <h2 className="text-sm font-semibold">{title}</h2>
        <div className={`grid grid-cols-1 gap-3 sm:grid-cols-2 ${columns}`}>
          {cards.map((stage) => (
            <StageCard
              key={stage.id}
              stage={stage}
              selected={stage.id === selectedId}
              onSelect={() => select(stage.id ?? null)}
            />
          ))}
        </div>
      </section>
    );
  };

  return (
    <MonitorFrame
      hasData={Boolean(summary)}
      isLoading={query.isLoading}
      isFetching={query.isFetching}
      error={query.isError ? t("monitor.stats.fetchError") : undefined}
      onRefresh={() => void query.refetch()}
      updatedAt={query.dataUpdatedAt}
      actions={
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={() => setDiagnosticsOpen(true)}
        >
          <Stethoscope className="size-4" aria-hidden />
          {t("monitor.processing.diagnostics.title", "Diagnostics")}
        </button>
      }
    >
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-5">
          {group("media", t("monitor.processing.groups.media", "Media"), "xl:grid-cols-3")}
          {group("catalog", t("monitor.processing.groups.catalog", "Catalog"), "xl:grid-cols-4")}
        </div>
        <aside className="card h-fit border border-base-300 bg-base-100 p-5">
          {selected ? (
            <StageDetailPanel key={selected.id} stage={selected} onBack={() => select(null)} />
          ) : (
            <OverviewPanel overview={summary?.overview} />
          )}
        </aside>
      </div>
      <DiagnosticsDialog open={diagnosticsOpen} onClose={() => setDiagnosticsOpen(false)} />
    </MonitorFrame>
  );
}

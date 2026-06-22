import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { ComponentType, ReactNode } from "react";
import {
  Blocks,
  CalendarClock,
  GripVertical,
  Images,
  LayoutDashboard,
  type LucideIcon,
  MessageSquare,
  Pin,
  Sparkles,
  X,
} from "lucide-react";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { listWidgets } from "../widgets/registry";
import { getMockWidgetDataset, mockWidgetDatasets } from "../widgets/mockWidgetData";
import type { WidgetProps, WidgetSource, WidgetVariant } from "../widgets/types";

/** Icon + emitting tool per widget type. Labels/blurbs come from i18n. */
const WIDGET_ICONS: Record<string, LucideIcon> = {
  asset_grid: Images,
  facet_dashboard: LayoutDashboard,
  timeline: CalendarClock,
  storyline: Sparkles,
};
const WIDGET_TOOL: Record<string, string> = {
  asset_grid: "show",
  facet_dashboard: "show_facets",
  timeline: "show_timeline",
  storyline: "show_storyline",
};

/** Approx. board row height (px) to preview a widget at its grid-layout size. */
const BOARD_ROW = 92;

export default function WidgetLibrary() {
  const { t } = useI18n();
  const widgets = useMemo(() => listWidgets(), []);
  const [selectedType, setSelectedType] = useState(
    () => widgets.find((w) => w.type === "storyline")?.type ?? widgets[0]?.type ?? "",
  );
  const [mockId, setMockId] = useState(mockWidgetDatasets[0].id);
  const [variant, setVariant] = useState<WidgetVariant>("board");

  // i18n lookups (static keys so the extractor keeps them).
  const labels: Record<string, string> = {
    asset_grid: t("lumilio.widgetLibrary.widgets.assetGrid.label", "Asset grid"),
    facet_dashboard: t("lumilio.widgetLibrary.widgets.facetDashboard.label", "Facet dashboard"),
    timeline: t("lumilio.widgetLibrary.widgets.timeline.label", "Adaptive timeline"),
    storyline: t("lumilio.widgetLibrary.widgets.storyline.label", "Storyline"),
  };
  const blurbs: Record<string, string> = {
    asset_grid: t("lumilio.widgetLibrary.widgets.assetGrid.blurb", "A photo set: a compact grid inline, an infinite grid on the board."),
    facet_dashboard: t("lumilio.widgetLibrary.widgets.facetDashboard.blurb", "An album-style overview: cover, range, people, places and camera."),
    timeline: t("lumilio.widgetLibrary.widgets.timeline.blurb", "A draggable time-river — scrub across it to bring each moment into focus."),
    storyline: t("lumilio.widgetLibrary.widgets.storyline.blurb", "An Instagram-style story player for trips and moments."),
  };
  const datasetTitles: Record<string, string> = {
    "travel-day": t("lumilio.widgetLibrary.datasets.travelDay", "Kyoto day trip"),
    "travel-month": t("lumilio.widgetLibrary.datasets.travelMonth", "September album"),
    "archive-years": t("lumilio.widgetLibrary.datasets.archiveYears", "Family archive"),
  };

  const labelOf = (type: string) => labels[type] ?? type;
  const datasetTitleOf = (id: string) => datasetTitles[id] ?? getMockWidgetDataset(id).title;

  const selected = widgets.find((w) => w.type === selectedType) ?? widgets[0];
  const dataset = getMockWidgetDataset(mockId);
  const source = useMemo<WidgetSource>(() => ({ kind: "mock", mockId }), [mockId]);
  const facets = dataset.metadata.facets;
  const previewTitle = datasetTitleOf(mockId);
  const SelectedIcon = WIDGET_ICONS[selected?.type ?? ""] ?? Blocks;

  return (
    <div className="flex h-full flex-col bg-base-100">
      <PageHeader
        icon={<Blocks size={22} strokeWidth={1.5} className="text-primary" />}
        title={t("lumilio.widgetLibrary.title", "Widget Library")}
        subtitle={t(
          "lumilio.widgetLibrary.subtitle",
          "Preview every Lumilio agent widget against mock data.",
        )}
      >
        <Link className="btn btn-sm btn-ghost gap-1.5" to="/lumilio">
          <MessageSquare size={15} />
          {t("lumilio.widgetLibrary.backToLumilio", "Back to Lumilio")}
        </Link>
      </PageHeader>

      <div className="flex min-h-0 flex-1 flex-col border-t border-base-300 lg:flex-row">
        {/* Catalog rail */}
        <aside className="shrink-0 overflow-x-auto border-b border-base-300 lg:w-72 lg:overflow-y-auto lg:border-b-0 lg:border-r">
          <div className="px-4 pt-4 text-xs font-semibold uppercase tracking-wide text-base-content/45">
            {t("lumilio.widgetLibrary.catalog", "Widgets")}
          </div>
          <div className="flex gap-2 p-3 lg:flex-col">
            {widgets.map((definition) => {
              const Icon = WIDGET_ICONS[definition.type] ?? Blocks;
              const active = definition.type === selected?.type;
              return (
                <button
                  key={definition.type}
                  type="button"
                  onClick={() => setSelectedType(definition.type)}
                  className={`flex w-60 shrink-0 items-center gap-3 rounded-xl border p-3 text-left transition-colors lg:w-auto ${
                    active
                      ? "border-primary/40 bg-primary/10"
                      : "border-base-300 hover:border-base-content/20 hover:bg-base-200/50"
                  }`}
                >
                  <span
                    className={`grid size-9 shrink-0 place-items-center rounded-lg ${
                      active ? "bg-primary/20 text-primary" : "bg-base-200 text-base-content/55"
                    }`}
                  >
                    <Icon size={18} strokeWidth={1.6} />
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold text-base-content">
                      {labelOf(definition.type)}
                    </span>
                    <code className="block truncate font-mono text-[11px] text-base-content/45">
                      {WIDGET_TOOL[definition.type] ?? "show"}
                    </code>
                  </span>
                </button>
              );
            })}
          </div>
        </aside>

        {/* Inspector stage */}
        <main className="min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-5xl space-y-5 p-4 sm:p-6">
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2.5">
                <SelectedIcon size={22} strokeWidth={1.6} className="text-primary" />
                <h2 className="text-xl font-semibold text-base-content">
                  {labelOf(selected?.type ?? "")}
                </h2>
                <code className="rounded-md bg-base-200 px-2 py-0.5 font-mono text-xs text-base-content/60">
                  {WIDGET_TOOL[selected?.type ?? ""] ?? "show"}
                </code>
                {selected && (
                  <span className="badge badge-ghost badge-sm font-mono">
                    {selected.defaultLayout.w}×{selected.defaultLayout.h}
                  </span>
                )}
              </div>
              <p className="max-w-2xl text-sm text-base-content/60">{blurbs[selected?.type ?? ""]}</p>
            </div>

            {/* Controls: dataset + variant toggle */}
            <div className="flex flex-wrap items-center gap-3 rounded-xl border border-base-300 bg-base-200/30 p-2.5">
              <div className="inline-flex flex-wrap gap-1 rounded-lg border border-base-300 bg-base-100 p-1">
                {mockWidgetDatasets.map((item) => {
                  const active = item.id === mockId;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setMockId(item.id)}
                      className={`rounded-md px-3 py-1.5 text-left text-xs transition-colors ${
                        active
                          ? "bg-primary text-primary-content"
                          : "text-base-content/70 hover:bg-base-200"
                      }`}
                    >
                      <span className="block font-medium">{datasetTitleOf(item.id)}</span>
                      <span
                        className={`block font-mono text-[10px] ${
                          active ? "text-primary-content/75" : "text-base-content/40"
                        }`}
                      >
                        {item.metadata.facets?.histogram_granularity}
                      </span>
                    </button>
                  );
                })}
              </div>

              <div role="tablist" className="ml-auto inline-flex rounded-lg border border-base-300 bg-base-100 p-1">
                <ToggleTab active={variant === "inline"} onClick={() => setVariant("inline")} icon={<MessageSquare size={14} />}>
                  {t("lumilio.widgetLibrary.inline", "Inline")}
                </ToggleTab>
                <ToggleTab active={variant === "board"} onClick={() => setVariant("board")} icon={<LayoutDashboard size={14} />}>
                  {t("lumilio.widgetLibrary.board", "Board")}
                </ToggleTab>
              </div>
            </div>

            {/* Single preview */}
            {selected &&
              (variant === "inline" ? (
                <div className="rounded-xl border border-base-300 bg-base-200/40 p-4">
                  <selected.Component
                    source={source}
                    variant="inline"
                    count={dataset.count}
                    title={previewTitle}
                  />
                </div>
              ) : (
                <BoardFrame
                  Component={selected.Component}
                  source={source}
                  count={dataset.count}
                  title={previewTitle}
                  height={selected.defaultLayout.h * BOARD_ROW}
                />
              ))}

            <div className="flex flex-wrap items-center gap-1.5 text-xs text-base-content/45">
              <Stat label={t("lumilio.widgetLibrary.stat.assets", "assets")}>{dataset.count}</Stat>
              <Stat label={t("lumilio.widgetLibrary.stat.granularity", "buckets")}>
                {facets?.histogram_granularity ?? "—"}
              </Stat>
              <Stat label={t("lumilio.widgetLibrary.stat.places", "places")}>
                {facets?.top_places?.length ?? 0}
              </Stat>
              <Stat label={t("lumilio.widgetLibrary.stat.people", "people")}>
                {facets?.top_people?.length ?? 0}
              </Stat>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

function ToggleTab({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
        active ? "bg-primary text-primary-content" : "text-base-content/70 hover:bg-base-200"
      }`}
    >
      {icon}
      {children}
    </button>
  );
}

function Stat({ label, children }: { label: string; children: ReactNode }) {
  return (
    <span className="inline-flex items-baseline gap-1.5 rounded-md bg-base-200/60 px-2.5 py-1">
      <span className="font-mono font-semibold text-base-content">{children}</span>
      <span>{label}</span>
    </span>
  );
}

/** A faux board cell — title bar + body sized to the widget's grid footprint. */
function BoardFrame({
  Component,
  source,
  count,
  title,
  height,
}: {
  Component: ComponentType<WidgetProps>;
  source: WidgetSource;
  count: number;
  title: string;
  height: number;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-base-300 bg-base-100">
      <div className="flex items-center gap-2 border-b border-base-300 bg-base-200/50 px-3 py-2">
        <GripVertical size={14} className="text-base-content/30" />
        <span className="truncate text-xs font-medium text-base-content/70">{title}</span>
        <span className="ml-auto flex items-center gap-1.5 text-base-content/30">
          <Pin size={13} />
          <X size={13} />
        </span>
      </div>
      <div style={{ height }} className="min-h-0">
        <Component source={source} variant="board" count={count} title={title} />
      </div>
    </div>
  );
}

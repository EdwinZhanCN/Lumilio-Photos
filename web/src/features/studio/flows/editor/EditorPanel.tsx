import React from "react";
import type { TFunction } from "i18next";
import { Crop, Frame, MousePointerClick, SlidersHorizontal, Type, Undo2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { StudioEditAdjustments } from "../../model/editTypes";
import type { AdjustmentKey } from "../../model/developConfig";
import type { CanvasSpec } from "../../model/canvasSpec";
import type { Layer } from "../../model/layers";
import { DevelopSections } from "./develop/DevelopSections";
import { FramePanel } from "./frame/FramePanel";
import { TextPanel } from "./text/TextPanel";
import { CropPanel } from "./crop/CropPanel";
import type { DepthStatus } from "./StudioEditor";

export type EditorTab = "develop" | "crop" | "frame" | "text";

type EditorPanelProps = {
  tab: EditorTab;
  onTabChange: (tab: EditorTab) => void;

  adjustments: StudioEditAdjustments;
  onAdjustmentChange: (key: AdjustmentKey, value: number) => void;
  onGeometryChange: (
    key: "rotation" | "flipHorizontal" | "flipVertical",
    value: number | boolean,
  ) => void;
  onResetAll: () => void;

  canvas: CanvasSpec | null;
  layers: readonly Layer[];
  activeTemplateId: string | null;
  templatePreviews: ReadonlyMap<string, string>;
  exifAvailable: boolean;
  selectedLayerId: string | null;
  onApplyTemplate: (templateId: string) => void;
  onClearTemplate: () => void;
  onCanvasChange: (next: CanvasSpec) => void;
  onLayersChange: (next: Layer[]) => void;
  onSelectLayer: (layerId: string | null) => void;

  cropAspectKey: string;
  onCropAspectChange: (key: string) => void;
  onResetCrop: () => void;

  depthStatus: DepthStatus;
  depthFeather: number;
  onGenerateDepth: () => void;
  onDepthFeatherChange: (value: number) => void;

  disabled?: boolean;
  /** Desktop rail visibility. */
  open?: boolean;
  /** Below `lg` the panel becomes a bottom sheet; these control its visibility. */
  mobileOpen?: boolean;
  onMobileClose?: () => void;
};

const TABS: Array<{ id: EditorTab; icon: typeof SlidersHorizontal }> = [
  { id: "develop", icon: SlidersHorizontal },
  { id: "crop", icon: Crop },
  { id: "frame", icon: Frame },
  { id: "text", icon: Type },
];

/** Spelled out so the i18n extractor can see the keys; it cannot read a computed one. */
function tabLabel(t: TFunction, tab: EditorTab): string {
  switch (tab) {
    case "crop":
      return t("studio.tab.crop", { defaultValue: "Crop" });
    case "frame":
      return t("studio.tab.frame", { defaultValue: "Frame" });
    case "text":
      return t("studio.tab.text", { defaultValue: "Text" });
    default:
      return t("studio.tab.develop", { defaultValue: "Develop" });
  }
}

/**
 * The editor's right-hand panel: develop adjustments, frame presets, and text
 * layers under one shell.
 *
 * Develop transforms the photo's pixels; frame and text compose around and on
 * top of the result. They are separate tabs because they are separate stages
 * of the pipeline, and because a preset writes to both the canvas and the layer
 * stack — a user who has just applied one needs to find both halves.
 */
export function EditorPanel({
  tab,
  onTabChange,
  adjustments,
  onAdjustmentChange,
  onGeometryChange,
  onResetAll,
  canvas,
  layers,
  activeTemplateId,
  templatePreviews,
  exifAvailable,
  selectedLayerId,
  onApplyTemplate,
  onClearTemplate,
  onCanvasChange,
  onLayersChange,
  onSelectLayer,
  cropAspectKey,
  onCropAspectChange,
  onResetCrop,
  depthStatus,
  depthFeather,
  onGenerateDepth,
  onDepthFeatherChange,
  disabled = false,
  open = true,
  mobileOpen = false,
  onMobileClose,
}: EditorPanelProps): React.JSX.Element {
  const { t } = useI18n();
  const activeTab = TABS.find((entry) => entry.id === tab) ?? TABS[0];

  return (
    <>
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/40 lg:hidden"
          onClick={onMobileClose}
          aria-hidden="true"
        />
      )}
      <aside
        className={`${
          mobileOpen ? "flex" : "hidden"
        } fixed inset-x-0 bottom-0 z-50 max-h-[75vh] w-full shrink-0 flex-col rounded-t-2xl border-t border-base-300 bg-base-100 shadow-2xl lg:static lg:z-auto lg:h-full lg:max-h-none lg:w-[340px] lg:rounded-none lg:border-t-0 lg:border-l lg:bg-base-200/40 lg:shadow-none ${
          open ? "lg:flex" : "lg:hidden"
        }`}
        aria-label={t("studio.panel.edit", { defaultValue: "Editor panel" })}
      >
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-base-content">
            <activeTab.icon size={16} className="text-base-content/70" />
            {tabLabel(t, activeTab.id)}
          </h2>
          <div className="flex items-center gap-1.5">
            {tab === "develop" && (
              <div
                className="tooltip tooltip-left"
                data-tip={t("studio.develop.resetAll", { defaultValue: "Reset all adjustments" })}
              >
                <button
                  type="button"
                  onClick={onResetAll}
                  disabled={disabled}
                  className="btn btn-ghost btn-xs gap-1 text-base-content/60"
                >
                  <Undo2 size={13} />
                  {t("studio.develop.reset", { defaultValue: "Reset" })}
                </button>
              </div>
            )}
            <button
              type="button"
              onClick={onMobileClose}
              aria-label={t("common.close", { defaultValue: "Close" })}
              className="btn btn-ghost btn-xs btn-square text-base-content/60 lg:hidden"
            >
              <X size={15} />
            </button>
          </div>
        </div>

        <div className="flex gap-1.5 border-b border-base-300 px-3 py-2">
          {TABS.map(({ id, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => onTabChange(id)}
              aria-pressed={tab === id}
              className={`btn btn-sm flex-1 gap-1.5 border-base-300 bg-base-100 text-[12px] ${
                tab === id
                  ? "btn-active border-primary/50 text-primary"
                  : "text-base-content/70 hover:border-base-content/25"
              }`}
            >
              <Icon className="h-3.5 w-3.5" />
              {tabLabel(t, id)}
            </button>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-3">
          {tab === "develop" && (
            <DevelopSections
              adjustments={adjustments}
              disabled={disabled}
              onAdjustmentChange={onAdjustmentChange}
              onGeometryChange={onGeometryChange}
            />
          )}
          {tab === "crop" && (
            <CropPanel
              aspectKey={cropAspectKey}
              onAspectChange={onCropAspectChange}
              onReset={onResetCrop}
              disabled={disabled}
            />
          )}
          {tab === "frame" && (
            <FramePanel
              canvas={canvas}
              activeTemplateId={activeTemplateId}
              templatePreviews={templatePreviews}
              exifAvailable={exifAvailable}
              disabled={disabled}
              onApplyTemplate={onApplyTemplate}
              onClearTemplate={onClearTemplate}
              onCanvasChange={onCanvasChange}
            />
          )}
          {tab === "text" && (
            <TextPanel
              layers={layers}
              selectedLayerId={selectedLayerId}
              disabled={disabled}
              onSelectLayer={onSelectLayer}
              onLayersChange={onLayersChange}
              depthStatus={depthStatus}
              depthFeather={depthFeather}
              onGenerateDepth={onGenerateDepth}
              onDepthFeatherChange={onDepthFeatherChange}
            />
          )}
          <div className="h-4" />
        </div>

        {tab === "develop" && (
          <div className="border-t border-base-300 px-4 py-2.5">
            <p className="flex items-center gap-1.5 text-[11px] text-base-content/40">
              <MousePointerClick size={12} />
              {t("studio.develop.hint", { defaultValue: "Double-click any slider to reset it." })}
            </p>
          </div>
        )}
      </aside>
    </>
  );
}

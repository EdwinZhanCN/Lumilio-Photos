import React, { useState } from "react";
import {
  Crop,
  FlipHorizontal2,
  FlipVertical2,
  MousePointerClick,
  RotateCcw,
  RotateCw,
  SlidersHorizontal,
  Undo2,
  X,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { DEFAULT_STUDIO_ADJUSTMENTS, type StudioEditAdjustments } from "../editor/runtime/types";
import {
  DEVELOP_GROUPS,
  isGeometryModified,
  isGroupModified,
  type AdjustmentKey,
} from "../editor/developConfig";
import type { BorderExifSummary } from "../tools/border/BorderPanel";
import { SectionHeader } from "./SectionHeader";
import { SliderRow } from "./SliderRow";
import { BorderToolSection } from "./BorderToolSection";

type DevelopPanelProps = {
  adjustments: StudioEditAdjustments;
  disabled?: boolean;
  focusTools?: boolean;
  onAdjustmentChange: (key: AdjustmentKey, value: number) => void;
  onGeometryChange: (
    key: "rotation" | "flipHorizontal" | "flipVertical",
    value: number | boolean,
  ) => void;
  onResetAll: () => void;
  // Border tool
  borderParams: Record<string, unknown>;
  onBorderParamsChange: (next: Record<string, unknown>) => void;
  onApplyBorder: () => void;
  onClearBorder: () => void;
  isApplyingBorder: boolean;
  hasBorderResult: boolean;
  borderExifSummary?: BorderExifSummary;
  /** Below `lg` the panel becomes a bottom sheet; these control its visibility. */
  mobileOpen?: boolean;
  onMobileClose?: () => void;
};

type GroupId = "geometry" | "light" | "color" | "detail" | "tools";

function GeoButton({
  icon: Icon,
  label,
  active = false,
  disabled = false,
  onClick,
}: {
  icon: typeof RotateCcw;
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
}): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      disabled={disabled}
      className={`btn btn-sm flex-1 border-base-300 bg-base-100 ${
        active
          ? "btn-active border-primary/50 text-primary"
          : "text-base-content/70 hover:border-base-content/25"
      }`}
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}

export function DevelopPanel({
  adjustments,
  disabled = false,
  focusTools = false,
  onAdjustmentChange,
  onGeometryChange,
  onResetAll,
  borderParams,
  onBorderParamsChange,
  onApplyBorder,
  onClearBorder,
  isApplyingBorder,
  hasBorderResult,
  borderExifSummary,
  mobileOpen = false,
  onMobileClose,
}: DevelopPanelProps): React.JSX.Element {
  const { t } = useI18n();
  const [openMap, setOpenMap] = useState<Record<GroupId, boolean>>({
    geometry: !focusTools,
    light: !focusTools,
    color: true,
    detail: false,
    tools: focusTools,
  });
  const toggle = (id: GroupId) => setOpenMap((m) => ({ ...m, [id]: !m[id] }));
  const groupI18nKeys: Record<string, string> = {
    light: "studio.develop.light",
    color: "studio.develop.color",
    detail: "studio.develop.detail",
  };

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
        } fixed inset-x-0 bottom-0 z-50 max-h-[75vh] w-full shrink-0 flex-col rounded-t-2xl border-t border-base-300 bg-base-100 shadow-2xl lg:static lg:z-auto lg:flex lg:h-full lg:max-h-none lg:w-[340px] lg:rounded-none lg:border-t-0 lg:border-l lg:bg-base-200/40 lg:shadow-none`}
        aria-label="Develop panel"
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-3">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-base-content">
            <SlidersHorizontal size={16} className="text-base-content/70" />
            {t("studio.develop.title", { defaultValue: "Develop" })}
          </h2>
          <div className="flex items-center gap-1.5">
            <div
              className="tooltip tooltip-left"
              data-tip={t("studio.develop.resetAll", {
                defaultValue: "Reset all adjustments",
              })}
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

        {/* Scrollable groups */}
        <div className="min-h-0 flex-1 overflow-y-auto px-3">
          {/* Geometry */}
          <div className="border-b border-base-300">
            <SectionHeader
              icon={Crop}
              title={t("studio.develop.geometry", { defaultValue: "Geometry" })}
              open={openMap.geometry}
              modified={isGeometryModified(adjustments)}
              onToggle={() => toggle("geometry")}
            />
            {openMap.geometry && (
              <div className="pb-3.5 pt-1">
                <div className="mb-2 flex gap-1.5">
                  <GeoButton
                    icon={RotateCcw}
                    label="Rotate left"
                    disabled={disabled}
                    onClick={() =>
                      onGeometryChange("rotation", (adjustments.rotation - 90 + 360) % 360)
                    }
                  />
                  <GeoButton
                    icon={RotateCw}
                    label="Rotate right"
                    disabled={disabled}
                    onClick={() => onGeometryChange("rotation", (adjustments.rotation + 90) % 360)}
                  />
                  <GeoButton
                    icon={FlipHorizontal2}
                    label="Flip horizontal"
                    active={adjustments.flipHorizontal}
                    disabled={disabled}
                    onClick={() => onGeometryChange("flipHorizontal", !adjustments.flipHorizontal)}
                  />
                  <GeoButton
                    icon={FlipVertical2}
                    label="Flip vertical"
                    active={adjustments.flipVertical}
                    disabled={disabled}
                    onClick={() => onGeometryChange("flipVertical", !adjustments.flipVertical)}
                  />
                </div>
                <div className="flex items-center justify-between px-1 text-[11px] text-base-content/45">
                  <span>{t("studio.develop.rotation", { defaultValue: "Rotation" })}</span>
                  <span className="font-mono tabular-nums">{adjustments.rotation}°</span>
                </div>
              </div>
            )}
          </div>

          {/* Light / Color / Detail */}
          {DEVELOP_GROUPS.map((group) => (
            <div key={group.id} className="border-b border-base-300">
              <SectionHeader
                icon={group.icon}
                title={t(groupI18nKeys[group.id], {
                  defaultValue: group.title,
                })}
                open={openMap[group.id]}
                modified={isGroupModified(group, adjustments)}
                onToggle={() => toggle(group.id)}
              />
              {openMap[group.id] && (
                <div className="pb-3 pl-1 pr-1">
                  {group.controls.map((control) => (
                    <SliderRow
                      key={control.key}
                      control={control}
                      value={adjustments[control.key]}
                      defaultValue={DEFAULT_STUDIO_ADJUSTMENTS[control.key]}
                      disabled={disabled}
                      onChange={(v) => onAdjustmentChange(control.key, v)}
                    />
                  ))}
                </div>
              )}
            </div>
          ))}

          {/* Tools (Border) */}
          <BorderToolSection
            open={openMap.tools}
            onToggle={() => toggle("tools")}
            value={borderParams}
            onChange={onBorderParamsChange}
            onApply={onApplyBorder}
            onClear={onClearBorder}
            isApplying={isApplyingBorder}
            hasResult={hasBorderResult}
            disabled={disabled}
            exifSummary={borderExifSummary}
          />

          <div className="h-4" />
        </div>

        {/* Footer hint */}
        <div className="border-t border-base-300 px-4 py-2.5">
          <p className="flex items-center gap-1.5 text-[11px] text-base-content/40">
            <MousePointerClick size={12} />
            {t("studio.develop.hint", {
              defaultValue: "Double-click any slider to reset it.",
            })}
          </p>
        </div>
      </aside>
    </>
  );
}

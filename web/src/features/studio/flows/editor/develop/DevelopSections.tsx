import React, { useState } from "react";
import {
  Crop,
  FlipHorizontal2,
  FlipVertical2,
  RotateCcw,
  RotateCw,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { DEFAULT_STUDIO_ADJUSTMENTS, type StudioEditAdjustments } from "../../../model/editTypes";
import {
  DEVELOP_GROUPS,
  isGeometryModified,
  isGroupModified,
  type AdjustmentKey,
} from "../../../model/developConfig";
import { SectionHeader } from "./SectionHeader";
import { SliderRow } from "./SliderRow";

type DevelopSectionsProps = {
  adjustments: StudioEditAdjustments;
  disabled?: boolean;
  onAdjustmentChange: (key: AdjustmentKey, value: number) => void;
  onGeometryChange: (
    key: "rotation" | "flipHorizontal" | "flipVertical",
    value: number | boolean,
  ) => void;
};

type GroupId = "geometry" | "light" | "color" | "detail";

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

/**
 * The develop adjustments: geometry plus the light/color/detail slider groups.
 *
 * Panel chrome (header, tabs, footer) belongs to {@link EditorPanel}; this
 * renders only the sections so Frame and Text can sit beside it under the same
 * shell.
 */
export function DevelopSections({
  adjustments,
  disabled = false,
  onAdjustmentChange,
  onGeometryChange,
}: DevelopSectionsProps): React.JSX.Element {
  const { t } = useI18n();
  const [openMap, setOpenMap] = useState<Record<GroupId, boolean>>({
    geometry: true,
    light: true,
    color: true,
    detail: false,
  });
  const toggle = (id: GroupId) => setOpenMap((m) => ({ ...m, [id]: !m[id] }));
  const groupI18nKeys: Record<string, string> = {
    light: "studio.develop.light",
    color: "studio.develop.color",
    detail: "studio.develop.detail",
  };

  return (
    <>
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
                onClick={() => onGeometryChange("rotation", (adjustments.rotation - 90 + 360) % 360)}
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

      {DEVELOP_GROUPS.map((group) => (
        <div key={group.id} className="border-b border-base-300">
          <SectionHeader
            icon={group.icon}
            title={t(groupI18nKeys[group.id], { defaultValue: group.title })}
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
    </>
  );
}

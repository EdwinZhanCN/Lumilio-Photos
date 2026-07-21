import React, { useState } from "react";
import { Frame, Image as ImageIcon, Palette, Sparkles } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import {
  DEFAULT_CANVAS,
  DEFAULT_FROSTED_BACKGROUND,
  type CanvasBackground,
  type CanvasSpec,
} from "../../../model/canvasSpec";
import { FRAME_TEMPLATES } from "../../../modules/frame/frameTemplates";
import { SectionHeader } from "../develop/SectionHeader";
import { CanvasControls } from "./CanvasControls";
import { TemplateGrid } from "./TemplateGrid";

type FramePanelProps = {
  canvas: CanvasSpec | null;
  /** Template whose layers are currently on the photo, if any. */
  activeTemplateId: string | null;
  /** Preview images keyed by template id; absent entries render as placeholders. */
  templatePreviews: ReadonlyMap<string, string>;
  /** EXIF-driven templates need a matched camera to say anything. */
  exifAvailable: boolean;
  disabled?: boolean;
  onApplyTemplate: (templateId: string) => void;
  onClearTemplate: () => void;
  onCanvasChange: (next: CanvasSpec) => void;
};

type GroupId = "templates" | "border";

/**
 * Frame: pick a preset, or build a border by hand.
 *
 * The two sections are the same underlying model. A template writes a canvas
 * and a set of layers; the border controls edit that canvas directly. Applying
 * a template and then adjusting its border is therefore not a special case —
 * it is the only case.
 */
export function FramePanel({
  canvas,
  activeTemplateId,
  templatePreviews,
  exifAvailable,
  disabled = false,
  onApplyTemplate,
  onClearTemplate,
  onCanvasChange,
}: FramePanelProps): React.JSX.Element {
  const { t } = useI18n();
  const [openMap, setOpenMap] = useState<Record<GroupId, boolean>>({
    templates: true,
    border: false,
  });
  const toggle = (id: GroupId) => setOpenMap((m) => ({ ...m, [id]: !m[id] }));

  const effectiveCanvas = canvas ?? DEFAULT_CANVAS;

  const setBackground = (background: CanvasBackground) =>
    onCanvasChange({ ...effectiveCanvas, background });

  return (
    <>
      <div className="border-b border-base-300">
        <SectionHeader
          icon={Frame}
          title={t("studio.frame.templates", { defaultValue: "Presets" })}
          open={openMap.templates}
          modified={activeTemplateId !== null}
          onToggle={() => toggle("templates")}
        />
        {openMap.templates && (
          <div className="space-y-2.5 pb-3.5 pl-1 pr-1 pt-1">
            {!exifAvailable && (
              <p className="text-[11px] text-base-content/45">
                {t("studio.frame.noExif", {
                  defaultValue:
                    "This photo has no camera EXIF, so presets that print camera details will come out sparse.",
                })}
              </p>
            )}
            <TemplateGrid
              templates={FRAME_TEMPLATES}
              activeTemplateId={activeTemplateId}
              previews={templatePreviews}
              disabled={disabled}
              onSelect={onApplyTemplate}
            />
            {activeTemplateId && (
              <button
                type="button"
                className="btn btn-ghost btn-sm w-full text-base-content/70"
                onClick={onClearTemplate}
                disabled={disabled}
              >
                {t("studio.frame.clear", { defaultValue: "Remove frame" })}
              </button>
            )}
          </div>
        )}
      </div>

      <div className="border-b border-base-300 last:border-0">
        <SectionHeader
          icon={Palette}
          title={t("studio.frame.border", { defaultValue: "Border" })}
          open={openMap.border}
          modified={canvas !== null}
          onToggle={() => toggle("border")}
        />
        {openMap.border && (
          <div className="space-y-3 pb-3.5 pl-1 pr-1 pt-1">
            <div className="flex gap-1.5">
              <BackgroundButton
                icon={Palette}
                label={t("studio.frame.solid", { defaultValue: "Solid" })}
                active={effectiveCanvas.background.kind === "solid"}
                disabled={disabled}
                onClick={() => setBackground({ kind: "solid", color: "#ffffff" })}
              />
              <BackgroundButton
                icon={ImageIcon}
                label={t("studio.frame.gradient", { defaultValue: "Gradient" })}
                active={effectiveCanvas.background.kind === "gradient"}
                disabled={disabled}
                onClick={() =>
                  setBackground({ kind: "gradient", from: "#ffffff", to: "#d4d4d4", angle: 180 })
                }
              />
              <BackgroundButton
                icon={Sparkles}
                label={t("studio.frame.frosted", { defaultValue: "Frosted" })}
                active={effectiveCanvas.background.kind === "frosted"}
                disabled={disabled}
                onClick={() => setBackground({ ...DEFAULT_FROSTED_BACKGROUND })}
              />
            </div>

            <CanvasControls
              canvas={effectiveCanvas}
              disabled={disabled}
              onChange={onCanvasChange}
            />
          </div>
        )}
      </div>
    </>
  );
}

function BackgroundButton({
  icon: Icon,
  label,
  active,
  disabled,
  onClick,
}: {
  icon: typeof Palette;
  label: string;
  active: boolean;
  disabled: boolean;
  onClick: () => void;
}): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`btn btn-sm flex-1 gap-1 border-base-300 bg-base-100 text-[11px] ${
        active
          ? "btn-active border-primary/50 text-primary"
          : "text-base-content/70 hover:border-base-content/25"
      }`}
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

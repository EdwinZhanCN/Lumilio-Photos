import React from "react";
import type { TFunction } from "i18next";
import {
  AlignCenter,
  AlignLeft,
  AlignRight,
  Copy,
  Italic,
  Plus,
  Trash2,
  Type,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { STUDIO_FONTS } from "../../../model/fonts";
import {
  createTextLayer,
  DEFAULT_SHADOW,
  displayText,
  isTextLayer,
  type Layer,
  type TextAlign,
  type TextCase,
  type TextLayer,
} from "../../../model/layers";
import { ValueSlider } from "../../../components/ValueSlider";

type TextPanelProps = {
  layers: readonly Layer[];
  selectedLayerId: string | null;
  disabled?: boolean;
  onSelectLayer: (layerId: string | null) => void;
  onLayersChange: (next: Layer[]) => void;
};

/**
 * Text layers: the stack, and the properties of whichever one is selected.
 *
 * Logo layers appear in the stack but are not editable here — they are placed
 * by templates and carry no typography. Showing them keeps the stack honest
 * about draw order, which is what the list represents.
 */
export function TextPanel({
  layers,
  selectedLayerId,
  disabled = false,
  onSelectLayer,
  onLayersChange,
}: TextPanelProps): React.JSX.Element {
  const { t } = useI18n();
  const selected = layers.find((layer) => layer.id === selectedLayerId) ?? null;

  const replaceLayer = (next: TextLayer) =>
    onLayersChange(layers.map((layer) => (layer.id === next.id ? next : layer)));

  const addLayer = () => {
    const layer = createTextLayer({
      text: t("studio.text.newLayer", { defaultValue: "Your text" }),
      y: 0.85,
      shadow: { ...DEFAULT_SHADOW },
    });
    onLayersChange([...layers, layer]);
    onSelectLayer(layer.id);
  };

  const duplicateLayer = (layer: TextLayer) => {
    const copy = createTextLayer({ ...layer, y: Math.min(0.98, layer.y + 0.05) });
    onLayersChange([...layers, copy]);
    onSelectLayer(copy.id);
  };

  const removeLayer = (layerId: string) => {
    onLayersChange(layers.filter((layer) => layer.id !== layerId));
    if (selectedLayerId === layerId) onSelectLayer(null);
  };

  return (
    <div className="space-y-3 px-1 py-3">
      <div className="space-y-1">
        {layers.length === 0 && (
          <p className="px-1 py-3 text-center text-[11px] text-base-content/45">
            {t("studio.text.empty", {
              defaultValue: "No text yet. Add a layer, or apply a frame preset.",
            })}
          </p>
        )}
        {layers.map((layer) => {
          const active = layer.id === selectedLayerId;
          const isText = isTextLayer(layer);
          return (
            <div
              key={layer.id}
              className={`group flex items-center gap-2 rounded-lg border px-2 py-1.5 ${
                active
                  ? "border-primary bg-primary/5"
                  : "border-base-300 bg-base-100 hover:border-base-content/25"
              }`}
            >
              <button
                type="button"
                onClick={() => onSelectLayer(isText ? layer.id : null)}
                disabled={disabled || !isText}
                className="flex min-w-0 flex-1 items-center gap-2 text-left"
              >
                <Type
                  size={13}
                  className={active ? "shrink-0 text-primary" : "shrink-0 text-base-content/40"}
                />
                <span className="truncate text-[12px] text-base-content/80">
                  {isText
                    ? displayText(layer) ||
                      t("studio.text.untitled", { defaultValue: "Empty text" })
                    : `${layer.brand} · ${layer.variant}`}
                </span>
              </button>
              {isText && (
                <button
                  type="button"
                  onClick={() => duplicateLayer(layer)}
                  disabled={disabled}
                  aria-label={t("studio.text.duplicate", { defaultValue: "Duplicate" })}
                  className="btn btn-ghost btn-xs btn-square text-base-content/50 opacity-0 group-hover:opacity-100"
                >
                  <Copy size={12} />
                </button>
              )}
              <button
                type="button"
                onClick={() => removeLayer(layer.id)}
                disabled={disabled}
                aria-label={t("common.delete", { defaultValue: "Delete" })}
                className="btn btn-ghost btn-xs btn-square text-base-content/50 opacity-0 group-hover:opacity-100"
              >
                <Trash2 size={12} />
              </button>
            </div>
          );
        })}
      </div>

      <button
        type="button"
        onClick={addLayer}
        disabled={disabled}
        className="btn btn-sm w-full gap-1.5 border-base-300 bg-base-100 text-base-content/80"
      >
        <Plus size={14} />
        {t("studio.text.add", { defaultValue: "Add text" })}
      </button>

      {selected && isTextLayer(selected) && (
        <TextLayerEditor
          layer={selected}
          disabled={disabled}
          onChange={replaceLayer}
        />
      )}
    </div>
  );
}

const CASE_OPTIONS: TextCase[] = ["none", "upper", "lower", "title"];

/** Spelled out so the i18n extractor can see the keys; it cannot read a computed one. */
function caseLabel(t: TFunction, textCase: TextCase): string {
  switch (textCase) {
    case "upper":
      return t("studio.text.caseUpper", { defaultValue: "AA" });
    case "lower":
      return t("studio.text.caseLower", { defaultValue: "aa" });
    case "title":
      return t("studio.text.caseTitle", { defaultValue: "Ab" });
    default:
      return t("studio.text.caseNone", { defaultValue: "Aa" });
  }
}

const ALIGN_OPTIONS: Array<{ value: TextAlign; icon: typeof AlignLeft }> = [
  { value: "left", icon: AlignLeft },
  { value: "center", icon: AlignCenter },
  { value: "right", icon: AlignRight },
];

function TextLayerEditor({
  layer,
  disabled,
  onChange,
}: {
  layer: TextLayer;
  disabled: boolean;
  onChange: (next: TextLayer) => void;
}): React.JSX.Element {
  const { t } = useI18n();
  const font = STUDIO_FONTS.find((f) => f.family === layer.font.family) ?? STUDIO_FONTS[0];

  const patch = (changes: Partial<TextLayer>) => onChange({ ...layer, ...changes });
  const patchFont = (changes: Partial<TextLayer["font"]>) =>
    onChange({ ...layer, font: { ...layer.font, ...changes } });

  return (
    <div className="space-y-2 border-t border-base-300 pt-3">
      <textarea
        value={layer.text}
        disabled={disabled}
        rows={2}
        onChange={(e) => patch({ text: e.target.value })}
        aria-label={t("studio.text.content", { defaultValue: "Text" })}
        className="textarea textarea-bordered w-full resize-none bg-base-100 text-[13px] focus:border-primary/60 focus:outline-none focus:ring-1 focus:ring-primary/60"
      />

      <div className="flex gap-1.5">
        <select
          value={layer.font.family}
          disabled={disabled}
          onChange={(e) => patchFont({ family: e.target.value })}
          aria-label={t("studio.text.font", { defaultValue: "Font" })}
          className="select select-bordered select-xs h-7 min-w-0 flex-1 bg-base-100 text-[12px]"
        >
          {STUDIO_FONTS.map((option) => (
            <option key={option.family} value={option.family}>
              {option.family}
            </option>
          ))}
        </select>
        <select
          value={layer.font.weight}
          disabled={disabled}
          onChange={(e) => patchFont({ weight: Number(e.target.value) })}
          aria-label={t("studio.text.weight", { defaultValue: "Weight" })}
          className="select select-bordered select-xs h-7 w-[72px] bg-base-100 font-mono text-[11px]"
        >
          {font.weights.map((weight) => (
            <option key={weight} value={weight}>
              {weight}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={() => patchFont({ italic: !layer.font.italic })}
          disabled={disabled || !font.hasItalic}
          aria-label={t("studio.text.italic", { defaultValue: "Italic" })}
          aria-pressed={layer.font.italic}
          className={`btn btn-xs h-7 w-8 border-base-300 bg-base-100 ${
            layer.font.italic ? "btn-active text-primary" : "text-base-content/60"
          }`}
        >
          <Italic size={12} />
        </button>
      </div>

      <div className="flex gap-1.5">
        <div className="join flex-1">
          {ALIGN_OPTIONS.map(({ value, icon: Icon }) => (
            <button
              key={value}
              type="button"
              onClick={() => patch({ align: value })}
              disabled={disabled}
              aria-label={value}
              aria-pressed={layer.align === value}
              className={`btn join-item btn-xs h-7 flex-1 border-base-300 bg-base-100 ${
                layer.align === value ? "btn-active text-primary" : "text-base-content/60"
              }`}
            >
              <Icon size={12} />
            </button>
          ))}
        </div>
        <div className="join flex-1">
          {CASE_OPTIONS.map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => patch({ textCase: option })}
              disabled={disabled}
              aria-pressed={layer.textCase === option}
              className={`btn join-item btn-xs h-7 flex-1 border-base-300 bg-base-100 text-[11px] ${
                layer.textCase === option ? "btn-active text-primary" : "text-base-content/60"
              }`}
            >
              {caseLabel(t, option)}
            </button>
          ))}
        </div>
      </div>

      <ValueSlider
        label={t("studio.text.size", { defaultValue: "Size" })}
        value={layer.font.size}
        defaultValue={0.06}
        min={0.005}
        max={0.4}
        step={0.002}
        modified={layer.font.size !== 0.06}
        disabled={disabled}
        onChange={(size) => patchFont({ size })}
      />
      <ValueSlider
        label={t("studio.text.tracking", { defaultValue: "Letter spacing" })}
        value={layer.font.tracking}
        defaultValue={0}
        min={-0.1}
        max={0.6}
        step={0.01}
        modified={layer.font.tracking !== 0}
        disabled={disabled}
        onChange={(tracking) => patchFont({ tracking })}
      />
      <ValueSlider
        label={t("studio.text.lineHeight", { defaultValue: "Line height" })}
        value={layer.font.lineHeight}
        defaultValue={1.2}
        min={0.7}
        max={3}
        step={0.05}
        modified={layer.font.lineHeight !== 1.2}
        disabled={disabled}
        onChange={(lineHeight) => patchFont({ lineHeight })}
      />

      <div className="flex items-center justify-between py-1.5">
        <span className="text-[13px] text-base-content/70">
          {t("studio.text.color", { defaultValue: "Color" })}
        </span>
        <input
          type="color"
          value={layer.fill.kind === "solid" ? layer.fill.color : layer.fill.from}
          disabled={disabled}
          onChange={(e) => patch({ fill: { kind: "solid", color: e.target.value, opacity: 1 } })}
          aria-label={t("studio.text.color", { defaultValue: "Color" })}
          className="h-6 w-12 cursor-pointer rounded border border-base-300 bg-base-100"
        />
      </div>

      <label className="flex items-center justify-between py-1">
        <span className="text-[13px] text-base-content/70">
          {t("studio.text.shadow", { defaultValue: "Shadow" })}
        </span>
        <input
          type="checkbox"
          checked={layer.shadow !== null}
          disabled={disabled}
          onChange={(e) => patch({ shadow: e.target.checked ? { ...DEFAULT_SHADOW } : null })}
          className="toggle toggle-primary toggle-xs"
        />
      </label>

      <label className="flex items-center justify-between py-1">
        <span className="text-[13px] text-base-content/70">
          {t("studio.text.outline", { defaultValue: "Outline" })}
        </span>
        <input
          type="checkbox"
          checked={layer.stroke !== null}
          disabled={disabled}
          onChange={(e) =>
            patch({ stroke: e.target.checked ? { color: "#000000", width: 0.002 } : null })
          }
          className="toggle toggle-primary toggle-xs"
        />
      </label>

      <ValueSlider
        label={t("studio.text.opacity", { defaultValue: "Opacity" })}
        value={layer.opacity}
        defaultValue={1}
        min={0}
        max={1}
        step={0.02}
        modified={layer.opacity !== 1}
        disabled={disabled}
        onChange={(opacity) => patch({ opacity })}
      />
      <ValueSlider
        label={t("studio.text.rotation", { defaultValue: "Rotation" })}
        value={layer.rotation}
        defaultValue={0}
        min={-180}
        max={180}
        step={1}
        unit="°"
        modified={layer.rotation !== 0}
        disabled={disabled}
        onChange={(rotation) => patch({ rotation })}
      />
      <ValueSlider
        label={t("studio.text.positionX", { defaultValue: "Horizontal" })}
        value={layer.x}
        defaultValue={0.5}
        min={0}
        max={1}
        step={0.005}
        modified={layer.x !== 0.5}
        disabled={disabled}
        onChange={(x) => patch({ x })}
      />
      <ValueSlider
        label={t("studio.text.positionY", { defaultValue: "Vertical" })}
        value={layer.y}
        defaultValue={0.5}
        min={0}
        max={1}
        step={0.005}
        modified={layer.y !== 0.5}
        disabled={disabled}
        onChange={(y) => patch({ y })}
      />
    </div>
  );
}

import React, { useMemo } from "react";
import { Download, Loader2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { ValueSlider } from "../../../components/ValueSlider";
import type { ExportSizeMode } from "../../../modules/rendering/coordinateSystem";

export type ExportFormat = "image/jpeg" | "image/png" | "image/webp";

export type ExportSettings = {
  format: ExportFormat;
  /** 0..1, used by JPEG and WebP; ignored by PNG. */
  quality: number;
  sizeMode: ExportSizeMode;
};

export const DEFAULT_EXPORT_SETTINGS: ExportSettings = {
  format: "image/jpeg",
  quality: 0.92,
  sizeMode: { kind: "original" },
};

type ExportPanelProps = {
  open: boolean;
  settings: ExportSettings;
  onChange: (next: ExportSettings) => void;
  /** Native long edge the export can reach (min of source and the working cap). */
  sourceLongEdge: number;
  sourceWidth: number;
  sourceHeight: number;
  isExporting: boolean;
  onExport: () => void;
  onClose: () => void;
};

const FORMATS: Array<{ value: ExportFormat; label: string; ext: string; hasQuality: boolean }> = [
  { value: "image/jpeg", label: "JPEG", ext: "jpg", hasQuality: true },
  { value: "image/png", label: "PNG", ext: "png", hasQuality: false },
  { value: "image/webp", label: "WebP", ext: "webp", hasQuality: true },
];

/** Long edge the current size mode resolves to, capped at the source. */
function resolveLongEdge(mode: ExportSizeMode, sourceLongEdge: number): number {
  if (mode.kind === "percent") {
    return Math.max(
      1,
      Math.round((sourceLongEdge * Math.min(100, Math.max(1, mode.percent))) / 100),
    );
  }
  if (mode.kind === "longEdge") {
    return Math.min(sourceLongEdge, Math.max(1, Math.round(mode.longEdge)));
  }
  return sourceLongEdge;
}

export function ExportPanel({
  open,
  settings,
  onChange,
  sourceLongEdge,
  sourceWidth,
  sourceHeight,
  isExporting,
  onExport,
  onClose,
}: ExportPanelProps): React.JSX.Element {
  const { t } = useI18n();
  const activeFormat = FORMATS.find((f) => f.value === settings.format) ?? FORMATS[0];

  const output = useMemo(() => {
    const longEdge = resolveLongEdge(settings.sizeMode, sourceLongEdge);
    const aspect = sourceWidth > 0 && sourceHeight > 0 ? sourceWidth / sourceHeight : 1;
    const landscape = sourceWidth >= sourceHeight;
    const width = landscape ? longEdge : Math.round(longEdge * aspect);
    const height = landscape ? Math.round(longEdge / aspect) : longEdge;
    return { width, height };
  }, [settings.sizeMode, sourceLongEdge, sourceWidth, sourceHeight]);

  const setFormat = (format: ExportFormat) => onChange({ ...settings, format });
  const setQuality = (percent: number) => onChange({ ...settings, quality: percent / 100 });
  const setSizeMode = (sizeMode: ExportSizeMode) => onChange({ ...settings, sizeMode });

  return (
    <dialog className={`modal ${open ? "modal-open" : ""}`}>
      <div className="modal-box max-w-md">
        <h3 className="text-base font-semibold">
          {t("studio.export.title", { defaultValue: "Export" })}
        </h3>

        {/* Format */}
        <div className="mt-4">
          <div className="mb-1.5 text-xs font-medium text-base-content/60">
            {t("studio.export.format", { defaultValue: "Format" })}
          </div>
          <div className="join w-full">
            {FORMATS.map((f) => (
              <button
                key={f.value}
                type="button"
                onClick={() => setFormat(f.value)}
                className={`btn join-item flex-1 btn-sm ${
                  settings.format === f.value ? "btn-primary" : "btn-ghost border-base-300"
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>

        {/* Quality (JPEG / WebP only) */}
        {activeFormat.hasQuality && (
          <div className="mt-4">
            <ValueSlider
              label={t("studio.export.quality", { defaultValue: "Quality" })}
              value={Math.round(settings.quality * 100)}
              defaultValue={92}
              min={50}
              max={100}
              step={1}
              unit="%"
              onChange={setQuality}
            />
          </div>
        )}

        {/* Size */}
        <div className="mt-4">
          <div className="mb-1.5 text-xs font-medium text-base-content/60">
            {t("studio.export.size", { defaultValue: "Size" })}
          </div>
          <div className="join w-full">
            <button
              type="button"
              onClick={() => setSizeMode({ kind: "original" })}
              className={`btn join-item flex-1 btn-sm ${
                settings.sizeMode.kind === "original" ? "btn-primary" : "btn-ghost border-base-300"
              }`}
            >
              {t("studio.export.original", { defaultValue: "Original" })}
            </button>
            <button
              type="button"
              onClick={() => setSizeMode({ kind: "percent", percent: 50 })}
              className={`btn join-item flex-1 btn-sm ${
                settings.sizeMode.kind === "percent" ? "btn-primary" : "btn-ghost border-base-300"
              }`}
            >
              {t("studio.export.percent", { defaultValue: "Percent" })}
            </button>
            <button
              type="button"
              onClick={() =>
                setSizeMode({ kind: "longEdge", longEdge: Math.min(2048, sourceLongEdge) })
              }
              className={`btn join-item flex-1 btn-sm ${
                settings.sizeMode.kind === "longEdge" ? "btn-primary" : "btn-ghost border-base-300"
              }`}
            >
              {t("studio.export.longEdge", { defaultValue: "Long edge" })}
            </button>
          </div>

          {settings.sizeMode.kind === "percent" && (
            <div className="mt-3">
              <ValueSlider
                label={t("studio.export.scale", { defaultValue: "Scale" })}
                value={settings.sizeMode.percent}
                defaultValue={50}
                min={10}
                max={100}
                step={5}
                unit="%"
                onChange={(percent) => setSizeMode({ kind: "percent", percent })}
              />
            </div>
          )}

          {settings.sizeMode.kind === "longEdge" && (
            <label className="mt-3 flex items-center gap-2 text-sm">
              <span className="text-base-content/60">
                {t("studio.export.longEdgePx", { defaultValue: "Long edge (px)" })}
              </span>
              <input
                type="number"
                min={64}
                max={sourceLongEdge}
                value={settings.sizeMode.longEdge}
                onChange={(e) =>
                  setSizeMode({ kind: "longEdge", longEdge: Number(e.target.value) || 64 })
                }
                className="input input-sm input-bordered w-28"
              />
            </label>
          )}
        </div>

        {/* Output estimate */}
        <div className="mt-4 rounded-lg bg-base-200 px-3 py-2 text-xs text-base-content/70">
          {t("studio.export.estimate", {
            defaultValue: "Approx. output",
          })}
          :{" "}
          <span className="font-mono tabular-nums">
            {output.width} × {output.height}
          </span>{" "}
          · {activeFormat.label}
        </div>

        <div className="modal-action mt-5">
          <button type="button" onClick={onClose} className="btn btn-ghost btn-sm">
            {t("studio.export.cancel", { defaultValue: "Cancel" })}
          </button>
          <button
            type="button"
            onClick={onExport}
            disabled={isExporting}
            className="btn btn-primary btn-sm gap-1.5"
          >
            {isExporting ? <Loader2 size={15} className="animate-spin" /> : <Download size={15} />}
            {t("studio.export.confirm", { defaultValue: "Export" })}
          </button>
        </div>
      </div>
      <button
        type="button"
        aria-label={t("studio.export.cancel", { defaultValue: "Cancel" })}
        className="modal-backdrop"
        onClick={onClose}
      />
    </dialog>
  );
}

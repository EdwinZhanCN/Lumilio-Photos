import { useEffect, useId, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { isAbortProblem, localizeAPIProblem } from "@/lib/http-commons/problem";
import { downloadPhotoBlob } from "@/lib/photo-export/download";
import {
  exportDimensions,
  photoExportFilename,
  PHOTO_EXPORT_MIME,
  type PhotoExportArtifact,
  type PhotoExportFormat,
  type PhotoExportSettings,
} from "@/lib/photo-export/model";

type Props = {
  open: boolean;
  settings: PhotoExportSettings;
  onChange: (settings: PhotoExportSettings) => void;
  formats: readonly PhotoExportFormat[];
  sourceWidth: number;
  sourceHeight: number;
  sourceLabel: string;
  metadataNote: string;
  onExport: (settings: PhotoExportSettings, signal: AbortSignal) => Promise<PhotoExportArtifact>;
  onClose: () => void;
  onNotice: (message: string) => void;
};

/** Presentation and download lifecycle only; callers own source acquisition/rendering. */
export function PhotoExportDialog({
  open,
  settings,
  onChange,
  formats,
  sourceWidth,
  sourceHeight,
  sourceLabel,
  metadataNote,
  onExport,
  onClose,
  onNotice,
}: Props) {
  const { t } = useI18n();
  const id = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const running = useRef<AbortController | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dimensions = exportDimensions(sourceWidth, sourceHeight, settings.sizeMode);
  const longEdge = Math.max(sourceWidth, sourceHeight, 1);
  useEffect(() => {
    if (open) {
      setError(null);
      if (!dialog.current?.open) dialog.current?.showModal();
    } else {
      running.current?.abort();
      dialog.current?.close();
    }
  }, [open]);
  useEffect(
    () => () => {
      running.current?.abort();
    },
    [],
  );

  const submit = async () => {
    if (running.current || !formats.includes(settings.format)) return;
    const controller = new AbortController();
    running.current = controller;
    setBusy(true);
    setError(null);
    try {
      const snapshot = structuredClone(settings);
      const artifact = await onExport(snapshot, controller.signal);
      if (controller.signal.aborted) return;
      if (artifact.blob.type !== PHOTO_EXPORT_MIME[snapshot.format])
        throw new Error("Unexpected export format");
      downloadPhotoBlob(artifact.blob, photoExportFilename(snapshot));
      onNotice(
        artifact.warnings?.length
          ? artifact.warnings.join(" ")
          : t("photoExport.completed", "Photo download started."),
      );
      onClose();
    } catch (failure) {
      if (!controller.signal.aborted && !isAbortProblem(failure))
        setError(
          localizeAPIProblem(
            failure,
            t,
            t("photoExport.failed", "Export failed. Your settings are kept; try again."),
          ),
        );
    } finally {
      if (running.current === controller) {
        running.current = null;
        setBusy(false);
      }
    }
  };

  return (
    <dialog
      ref={dialog}
      className="modal"
      aria-labelledby={id}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
    >
      <div className="modal-box max-w-md">
        <h3 id={id} className="text-base font-semibold">
          {t("photoExport.title", "Export photo")}
        </h3>
        <p className="mt-2 text-sm text-base-content/70">{sourceLabel}</p>
        <fieldset disabled={busy} className="mt-4 flex flex-col gap-4">
          <label className="form-control">
            <span id={`${id}-format`} className="label">
              {t("photoExport.format", "Format")}
            </span>
            <select
              className="select w-full"
              aria-labelledby={`${id}-format`}
              value={settings.format}
              onChange={(event) => {
                const format = formats.find((value) => value === event.target.value);
                if (format) onChange({ ...settings, format });
              }}
            >
              {formats.map((format) => (
                <option key={format} value={format}>
                  {format.toUpperCase()}
                </option>
              ))}
            </select>
          </label>
          {settings.format !== "png" && (
            <label className="form-control">
              <span className="label">
                {t("photoExport.quality", "Quality")} · {Math.round(settings.quality * 100)}%
              </span>
              <input
                type="range"
                className="range range-sm"
                min={1}
                max={100}
                value={Math.round(settings.quality * 100)}
                onChange={(event) =>
                  onChange({ ...settings, quality: Number(event.target.value) / 100 })
                }
              />
            </label>
          )}
          <label className="form-control">
            <span id={`${id}-size`} className="label">
              {t("photoExport.size", "Size")}
            </span>
            <select
              className="select w-full"
              aria-labelledby={`${id}-size`}
              value={settings.sizeMode.kind}
              onChange={(event) =>
                onChange({
                  ...settings,
                  sizeMode:
                    event.target.value === "percent"
                      ? { kind: "percent", percent: 50 }
                      : event.target.value === "longEdge"
                        ? {
                            kind: "longEdge",
                            longEdge: dimensions ? Math.min(2048, longEdge) : 2048,
                          }
                        : { kind: "original" },
                })
              }
            >
              <option value="original">{t("photoExport.maximum", "Maximum available")}</option>
              <option value="percent" disabled={!dimensions}>
                {t("photoExport.percent", "Percent")}
              </option>
              <option value="longEdge">{t("photoExport.longEdge", "Long edge")}</option>
            </select>
          </label>
          {settings.sizeMode.kind === "percent" && (
            <label className="form-control">
              <span className="label">{t("photoExport.scale", "Scale (%)")}</span>
              <input
                className="input w-full"
                type="number"
                min={1}
                max={100}
                value={settings.sizeMode.percent}
                onChange={(event) =>
                  onChange({
                    ...settings,
                    sizeMode: {
                      kind: "percent",
                      percent: Math.min(100, Math.max(1, Number(event.target.value) || 1)),
                    },
                  })
                }
              />
            </label>
          )}
          {settings.sizeMode.kind === "longEdge" && (
            <label className="form-control">
              <span className="label">{t("photoExport.longEdgePx", "Long edge (px)")}</span>
              <input
                className="input w-full"
                type="number"
                min={1}
                max={dimensions ? longEdge : 60000}
                value={settings.sizeMode.longEdge}
                onChange={(event) =>
                  onChange({
                    ...settings,
                    sizeMode: {
                      kind: "longEdge",
                      longEdge: Math.min(
                        dimensions ? longEdge : 60000,
                        Math.max(1, Number(event.target.value) || 1),
                      ),
                    },
                  })
                }
              />
            </label>
          )}
          <label className="form-control">
            <span className="label">{t("photoExport.filename", "File name")}</span>
            <input
              className="input w-full"
              value={settings.filename}
              onChange={(event) => onChange({ ...settings, filename: event.target.value })}
            />
          </label>
        </fieldset>
        <div className="mt-4 text-xs text-base-content/70" aria-live="polite">
          <p>
            {t("photoExport.output", "Estimated output")}:{" "}
            {dimensions
              ? `${dimensions.width} × ${dimensions.height}`
              : t("photoExport.sourceSize", "Source dimensions")}
          </p>
          <p className="mt-1 break-all">{photoExportFilename(settings)}</p>
          <p className="mt-2">{metadataNote}</p>
        </div>
        {error && (
          <p role="alert" className="mt-3 text-sm text-error">
            {error}
          </p>
        )}
        <div className="modal-action">
          <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={busy || !formats.includes(settings.format)}
            onClick={() => void submit()}
          >
            {busy ? t("photoExport.exporting", "Exporting…") : t("photoExport.confirm", "Export")}
          </button>
        </div>
      </div>
    </dialog>
  );
}

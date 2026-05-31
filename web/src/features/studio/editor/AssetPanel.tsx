import React from "react";
import {
  Aperture,
  Camera,
  Clock,
  Focus,
  Sun,
  Timer,
  type LucideIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { PhotoThumb } from "@/features/studio/shared/PhotoThumb";

export type AssetExifRow = { label: string; value: string };

type AssetPanelProps = {
  assetId: string | null;
  fileName: string;
  sizeText: string;
  dimensionsText: string;
  typeText: string;
  exifRows: AssetExifRow[];
};

const EXIF_ICONS: Record<string, LucideIcon> = {
  Camera: Camera,
  Lens: Aperture,
  ISO: Sun,
  Aperture: Aperture,
  Shutter: Timer,
  "Focal Length": Focus,
  Captured: Clock,
};

function MetaRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}): React.JSX.Element {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1">
      <span className="text-xs text-base-content/50">{label}</span>
      <span
        className={`truncate text-xs text-base-content/85 ${
          mono ? "font-mono" : "font-medium"
        }`}
      >
        {value}
      </span>
    </div>
  );
}

export function AssetPanel({
  assetId,
  fileName,
  sizeText,
  dimensionsText,
  typeText,
  exifRows,
}: AssetPanelProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <aside
      className="hidden h-full w-[260px] shrink-0 flex-col overflow-y-auto border-r border-base-300 bg-base-200/40 lg:flex"
      aria-label="Asset info"
    >
      <div className="border-b border-base-300 p-4">
        <PhotoThumb
          assetId={assetId}
          size="medium"
          alt={fileName}
          className="aspect-[3/2] w-full"
        />
      </div>

      <div className="border-b border-base-300 px-4 py-3">
        <h3 className="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-base-content/45">
          {t("studio.asset.file", { defaultValue: "File" })}
        </h3>
        <MetaRow label={t("studio.asset.name", { defaultValue: "Name" })} value={fileName} mono />
        <MetaRow label={t("studio.asset.size", { defaultValue: "Size" })} value={sizeText} />
        <MetaRow
          label={t("studio.asset.dim", { defaultValue: "Dimensions" })}
          value={dimensionsText}
          mono
        />
        <MetaRow label={t("studio.asset.type", { defaultValue: "Type" })} value={typeText} />
      </div>

      {exifRows.length > 0 && (
        <div className="px-4 py-3">
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-base-content/45">
            {t("studio.asset.exif", { defaultValue: "Capture · EXIF" })}
          </h3>
          <div className="flex flex-col gap-0.5">
            {exifRows.map((row) => {
              const Icon = EXIF_ICONS[row.label] ?? Aperture;
              return (
                <div
                  key={row.label}
                  className="flex items-center gap-2.5 rounded-md px-1.5 py-1.5 hover:bg-base-200/60"
                >
                  <div className="grid h-6 w-6 shrink-0 place-items-center rounded text-base-content/40">
                    <Icon size={14} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-[10px] uppercase tracking-wide text-base-content/40">
                      {row.label}
                    </div>
                    <div className="truncate text-xs font-medium text-base-content/85">
                      {row.value}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </aside>
  );
}

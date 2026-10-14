import { useState } from "react";
import { PhotoExportDialog } from "@/components/PhotoExportDialog/PhotoExportDialog";
import { useMessage } from "@/features/notifications";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { defaultPhotoExportSettings, PHOTO_EXPORT_FORMATS } from "@/lib/photo-export/model";
import { exportPhoto } from "../../api/exportPhoto";

export function AssetExportDialog({ asset, onClose }: { asset: Asset; onClose: () => void }) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [settings, setSettings] = useState(() =>
    defaultPhotoExportSettings(asset.original_filename),
  );
  return (
    <PhotoExportDialog
      open
      settings={settings}
      onChange={setSettings}
      formats={PHOTO_EXPORT_FORMATS}
      sourceWidth={asset.width ?? 0}
      sourceHeight={asset.height ?? 0}
      sourceLabel={t("photoExport.assetSource", "Exports this photo without Studio edits.")}
      metadataNote={t(
        "photoExport.serverMetadata",
        "Compatible metadata and color profiles are preserved when available in the source.",
      )}
      onExport={(snapshot, signal) => exportPhoto(asset, snapshot, signal)}
      onClose={onClose}
      onNotice={(message) => showMessage("info", message)}
    />
  );
}

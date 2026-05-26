import React from "react";
import { Paintbrush, Upload, Download } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";

type StudioHeaderProps = {
  onOpenFile: () => void;
  onExportImage?: () => void;
  hasExportImage?: boolean;
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  onFileChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
};

export function StudioHeader({
  onOpenFile,
  onExportImage,
  hasExportImage = false,
  fileInputRef,
  onFileChange,
}: StudioHeaderProps) {
  const { t } = useI18n();

  return (
    <PageHeader
      title={t("studio.title")}
      icon={<Paintbrush className="w-6 h-6 text-primary" />}
    >
      <input
        type="file"
        ref={fileInputRef}
        onChange={onFileChange}
        accept="image/jpeg, image/png, image/tiff, image/heic, image/heif, image/webp"
        className="hidden"
      />
      <div className="flex flex-wrap items-center justify-end gap-2">
        <button onClick={onOpenFile} className="btn btn-sm btn-primary">
          <Upload className="w-4 h-4 mr-1" />
          {t("studio.imgOpen")}
        </button>
        {hasExportImage && onExportImage && (
          <button onClick={onExportImage} className="btn btn-sm btn-outline">
            <Download className="w-4 h-4 mr-1" />
            {t("common.save")}
          </button>
        )}
      </div>
    </PageHeader>
  );
}

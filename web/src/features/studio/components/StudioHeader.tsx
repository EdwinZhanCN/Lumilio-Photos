import React from "react";
import { PaintBrushIcon, ArrowUpTrayIcon } from "@heroicons/react/24/outline";
import { PageHeader } from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";

type StudioHeaderProps = {
  onOpenFile: () => void;
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  onFileChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
};

export function StudioHeader({
  onOpenFile,
  fileInputRef,
  onFileChange,
}: StudioHeaderProps) {
  const { t } = useI18n();
  return (
    <PageHeader
      title={t("studio.title")}
      icon={<PaintBrushIcon className="w-6 h-6 text-primary" />}
    >
      <input
        type="file"
        ref={fileInputRef}
        onChange={onFileChange}
        accept="image/jpeg, image/png, image/tiff, image/heic, image/heif, image/webp"
        className="hidden"
      />
      <button onClick={onOpenFile} className="ml-5 btn btn-sm btn-primary">
        <ArrowUpTrayIcon className="w-4 h-4 mr-1" />
        {t("studio.imgOpen")}
      </button>
    </PageHeader>
  );
}

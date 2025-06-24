import React from "react";
import { PaintBrushIcon, ArrowUpTrayIcon } from "@heroicons/react/24/outline";

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
  return (
    <header className="py-2 px-4 border-b border-base-content/10 flex justify-between items-center flex-shrink-0">
      <div className="flex items-center space-x-2">
        <PaintBrushIcon className="w-6 h-6 text-primary" />
        <h1 className="text-xl font-bold">Studio</h1>
      </div>
      <div className="flex items-center space-x-2">
        <input
          type="file"
          ref={fileInputRef}
          onChange={onFileChange}
          accept="image/jpeg, image/png, image/tiff, image/heic, image/heif, image/webp"
          className="hidden"
        />
        <button onClick={onOpenFile} className="btn btn-sm btn-primary">
          <ArrowUpTrayIcon className="w-4 h-4 mr-1" />
          Open Image
        </button>
      </div>
    </header>
  );
}

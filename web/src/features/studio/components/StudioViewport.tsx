import { PhotoIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

type StudioViewportProps = {
  imageUrl: string | null;
  selectedFile: File | null;
  onOpenFile: () => void;
};

export function StudioViewport({
  imageUrl,
  selectedFile,
  onOpenFile,
}: StudioViewportProps) {
  const { t } = useI18n();
  return (
    <div className="flex-1 overflow-hidden bg-base-300 relative">
      {imageUrl ? (
        <>
          {/* Main image display area */}
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <img
              src={imageUrl}
              alt={selectedFile?.name || t("studio.previewAlt")}
              className="object-contain max-h-full max-w-full shadow-lg"
            />
          </div>

          {/* Image Info Overlay at the bottom */}
          <div className="absolute bottom-0 left-0 right-0 bg-base-300/80 backdrop-blur-sm p-2 text-xs">
            {selectedFile && (
              <>
                <span className="font-semibold mr-2">{selectedFile.name}</span>
                <span>{(selectedFile.size / 1024).toFixed(2)} KB</span>
              </>
            )}
          </div>
        </>
      ) : (
        // Placeholder view when no image is loaded
        <div className="flex-1 flex items-center justify-center h-full">
          <div className="text-center p-8">
            <PhotoIcon className="w-16 h-16 mx-auto text-base-content/30" />
            <p className="mt-4">{t("studio.emptyHint")}</p>
            <button onClick={onOpenFile} className="btn btn-primary mt-4">
              {t("studio.imgOpen")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

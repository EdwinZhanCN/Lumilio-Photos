import { useEffect, useMemo, type ReactNode } from "react";
import { ChevronLeft, ChevronRight, Download, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { shareUrls } from "../utils/shareUrls";
import type { components } from "@/lib/http-commons/schema.d.ts";

type PublicAssetDTO = components["schemas"]["dto.PublicAssetDTO"];

export interface PublicShareLightboxProps {
  token: string;
  assets: PublicAssetDTO[];
  activeAssetId: string;
  onNavigate: (assetId: string) => void;
  onClose: () => void;
  allowDownload: boolean;
  includeOriginals: boolean;
}

/**
 * Minimal full-screen previous/next viewer for the public share page. Not a
 * reuse of FullScreenCarousel — that component is built around the full
 * authenticated Asset type and bakes in like/delete/album/export actions that
 * make no sense (and would leak internal APIs) on a public, tokenless page.
 */
export function PublicShareLightbox({
  token,
  assets,
  activeAssetId,
  onNavigate,
  onClose,
  allowDownload,
  includeOriginals,
}: PublicShareLightboxProps): ReactNode {
  const { t } = useI18n();
  const index = assets.findIndex((a) => a.asset_id === activeAssetId);
  const asset = index >= 0 ? assets[index] : undefined;
  const prevId = index > 0 ? assets[index - 1]?.asset_id : undefined;
  const nextId = index >= 0 && index < assets.length - 1 ? assets[index + 1]?.asset_id : undefined;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      else if (e.key === "ArrowLeft" && prevId) onNavigate(prevId);
      else if (e.key === "ArrowRight" && nextId) onNavigate(nextId);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [nextId, onClose, onNavigate, prevId]);

  const canDownloadOriginal = allowDownload && includeOriginals;
  const downloadUrl = useMemo(
    () => (asset?.asset_id ? shareUrls.getOriginalUrl(token, asset.asset_id) : undefined),
    [asset?.asset_id, token],
  );

  if (!asset || !asset.asset_id) return null;

  return (
    <div className="fixed inset-0 z-[9990] flex flex-col bg-black/95">
      <div className="flex shrink-0 items-center justify-end gap-2 p-3">
        {canDownloadOriginal && downloadUrl && (
          <a
            href={downloadUrl}
            download
            className="btn btn-circle btn-ghost text-white hover:bg-white/10"
            aria-label={t("share.public.lightbox.download", "Download")}
          >
            <Download size={20} />
          </a>
        )}
        <button
          type="button"
          className="btn btn-circle btn-ghost text-white hover:bg-white/10"
          onClick={onClose}
          aria-label={t("common.close", "Close")}
        >
          <X size={22} />
        </button>
      </div>

      <div className="relative flex flex-1 items-center justify-center overflow-hidden px-4 pb-4">
        {prevId && (
          <button
            type="button"
            className="btn btn-circle btn-ghost absolute left-2 top-1/2 -translate-y-1/2 text-white hover:bg-white/10"
            onClick={() => onNavigate(prevId)}
            aria-label={t("common.previous", "Previous")}
          >
            <ChevronLeft size={28} />
          </button>
        )}

        {asset.type === "VIDEO" ? (
          <video
            key={asset.asset_id}
            src={shareUrls.getWebVideoUrl(token, asset.asset_id)}
            controls
            autoPlay
            className="max-h-full max-w-full"
          />
        ) : asset.type === "AUDIO" ? (
          <audio
            key={asset.asset_id}
            src={shareUrls.getWebAudioUrl(token, asset.asset_id)}
            controls
            autoPlay
            className="w-full max-w-md"
          />
        ) : (
          <img
            key={asset.asset_id}
            src={shareUrls.getThumbnailUrl(token, asset.asset_id, "large")}
            alt=""
            className="max-h-full max-w-full object-contain"
          />
        )}

        {nextId && (
          <button
            type="button"
            className="btn btn-circle btn-ghost absolute right-2 top-1/2 -translate-y-1/2 text-white hover:bg-white/10"
            onClick={() => onNavigate(nextId)}
            aria-label={t("common.next", "Next")}
          >
            <ChevronRight size={28} />
          </button>
        )}
      </div>
    </div>
  );
}

export default PublicShareLightbox;

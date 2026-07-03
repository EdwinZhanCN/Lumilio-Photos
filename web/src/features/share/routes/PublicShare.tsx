import { useState, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ImageOff } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { usePublicShareView } from "../hooks/usePublicShareView";
import { PublicShareHeader } from "../components/PublicShareHeader";
import { PublicShareGrid } from "../components/PublicShareGrid";
import { PublicShareLightbox } from "../components/PublicShareLightbox";
import { shareUrls } from "../utils/shareUrls";
import { filenameFromContentDisposition, triggerBlobDownload } from "../utils/downloadBlob";

/**
 * Public, unauthenticated share viewer mounted at /s/:token (and
 * /s/:token/:assetId for the lightbox). Rendered outside the authenticated
 * app shell — see App.tsx's routing restructure — so it never shows the
 * sidebar/navbar/chat dock and never makes authenticated API calls.
 */
export function PublicShare(): ReactNode {
  const { token, assetId } = useParams<{ token: string; assetId?: string }>();
  const navigate = useNavigate();
  const { t } = useI18n();
  const showMessage = useMessage();
  const view = usePublicShareView(token);
  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownloadAll = async () => {
    if (!token) return;
    setIsDownloading(true);
    try {
      const response = await fetch(shareUrls.getDownloadUrl(token), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });
      if (!response.ok) {
        throw new Error(`Download failed with status ${response.status}`);
      }
      const blob = await response.blob();
      const filename =
        filenameFromContentDisposition(response.headers.get("content-disposition")) ??
        "lumilio-share.zip";
      triggerBlobDownload(blob, filename);
    } catch (error) {
      console.error("Failed to download share:", error);
      showMessage("error", t("share.public.header.downloadError", "Download failed. Please try again."));
    } finally {
      setIsDownloading(false);
    }
  };

  if (!token || view.notFound) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center gap-3 bg-base-100 px-4 text-center">
        <ImageOff className="size-10 text-base-content/30" />
        <p className="text-base font-medium">
          {t("share.public.unavailable.title", "This link is no longer available")}
        </p>
        <p className="max-w-sm text-sm text-base-content/60">
          {t(
            "share.public.unavailable.body",
            "It may have expired or been revoked by the person who shared it.",
          )}
        </p>
      </div>
    );
  }

  if (view.isMetadataLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-100">
        <span className="loading loading-spinner loading-lg text-primary" />
      </div>
    );
  }

  return (
    <div className="flex min-h-dvh flex-col bg-base-100">
      <PublicShareHeader
        title={view.metadata?.title ?? ""}
        description={view.metadata?.description}
        assetCount={view.metadata?.asset_count ?? view.total}
        expiresAt={view.metadata?.expires_at}
        allowDownload={view.metadata?.allow_download ?? false}
        onDownloadAll={handleDownloadAll}
        isDownloading={isDownloading}
      />

      <main className="flex-1">
        <PublicShareGrid
          token={token}
          assets={view.assets}
          onOpen={(id) => navigate(`/s/${token}/${id}`)}
          onLoadMore={view.fetchMore}
          hasMore={view.hasMore}
          isLoadingMore={view.isLoadingMore}
        />
      </main>

      <footer className="border-t border-base-200 py-4 text-center text-xs text-base-content/50">
        {t("share.public.footer", "Shared with Lumilio Photos")}
      </footer>

      {assetId && (
        <PublicShareLightbox
          token={token}
          assets={view.assets}
          activeAssetId={assetId}
          onNavigate={(id) => navigate(`/s/${token}/${id}`, { replace: true })}
          onClose={() => navigate(`/s/${token}`)}
          allowDownload={view.metadata?.allow_download ?? false}
          includeOriginals={view.metadata?.include_originals ?? false}
        />
      )}
    </div>
  );
}

export default PublicShare;

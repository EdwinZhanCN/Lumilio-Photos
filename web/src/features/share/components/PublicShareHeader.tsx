import type { ReactNode } from "react";
import { Download, ImageIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

export interface PublicShareHeaderProps {
  title: string;
  description?: string;
  assetCount: number;
  expiresAt?: string;
  allowDownload: boolean;
  onDownloadAll: () => void;
  isDownloading?: boolean;
}

/** Top bar for the public share viewer: title, asset count, expiry state, and
 * the download-all action when the share owner enabled it. */
export function PublicShareHeader({
  title,
  description,
  assetCount,
  expiresAt,
  allowDownload,
  onDownloadAll,
  isDownloading = false,
}: PublicShareHeaderProps): ReactNode {
  const { t, i18n } = useI18n();

  const expiryLabel = expiresAt
    ? t("share.public.header.expires", {
        date: new Date(expiresAt).toLocaleDateString(i18n.resolvedLanguage || i18n.language),
        defaultValue: "Expires {{date}}",
      })
    : null;

  return (
    <header className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-3 border-b border-base-200 bg-base-100/90 px-4 py-3 backdrop-blur">
      <div className="min-w-0">
        <h1 className="truncate text-lg font-semibold">{title}</h1>
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-base-content/60">
          <span className="inline-flex items-center gap-1">
            <ImageIcon className="size-3.5" />
            {t("share.public.header.assetCount", { count: assetCount, defaultValue: "{{count}} items" })}
          </span>
          {expiryLabel && (
            <>
              <span aria-hidden>·</span>
              <span>{expiryLabel}</span>
            </>
          )}
        </div>
        {description && (
          <p className="mt-1 max-w-xl truncate text-sm text-base-content/70">{description}</p>
        )}
      </div>

      {allowDownload && (
        <button
          type="button"
          className="btn btn-primary btn-sm gap-1.5 shadow-none"
          onClick={onDownloadAll}
          disabled={isDownloading}
        >
          {isDownloading ? (
            <span className="loading loading-spinner loading-xs" />
          ) : (
            <Download className="size-4" />
          )}
          {t("share.public.header.downloadAll", "Download all")}
        </button>
      )}
    </header>
  );
}

export default PublicShareHeader;

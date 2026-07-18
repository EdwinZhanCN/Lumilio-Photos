import { useEffect, useMemo } from "react";
import { Layers, X } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { useAssetStackDetails } from "../../api/useAssetStackDetails";

interface StackDetailModalProps {
  asset: Asset;
  open: boolean;
  stackSize?: number;
  onClose: () => void;
}

const truncateAssetId = (assetId: string) => {
  if (assetId.length <= 12) return assetId;
  return `${assetId.slice(0, 8)}...${assetId.slice(-4)}`;
};

const memberCardClasses = (isCurrent: boolean) =>
  [
    "overflow-hidden rounded-[1.5rem] border bg-base-100/95 shadow-[0_20px_50px_-32px_rgba(15,23,42,0.45)] transition-shadow",
    isCurrent ? "border-primary/35 ring-1 ring-primary/20" : "border-base-300/70",
  ].join(" ");

export default function StackDetailModal({
  asset,
  open,
  stackSize,
  onClose,
}: StackDetailModalProps) {
  const { t } = useI18n();
  const stackQuery = useAssetStackDetails(asset.asset_id, open);

  useEffect(() => {
    if (!open) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose, open]);

  const members = useMemo(() => {
    const stackMembers = stackQuery.data?.stack?.members ?? [];
    if (stackMembers.length === 0) {
      return [];
    }

    const coverPosition = Math.min(...stackMembers.map((member) => member.position ?? 0));

    return stackMembers.map((member) => ({
      ...member,
      isCover: member.position === coverPosition,
      isCurrent: member.primary_asset_id === asset.asset_id,
    }));
  }, [asset.asset_id, stackQuery.data?.stack?.members]);

  const memberCount = stackQuery.data?.stack?.member_count ?? stackSize ?? 0;

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-[70]">
      <button
        type="button"
        aria-label={t("assets.stackDetail.close", {
          defaultValue: "Close stack details",
        })}
        className="absolute inset-0 bg-slate-950/60 backdrop-blur-sm"
        onClick={onClose}
      />

      <div className="relative flex min-h-full items-center justify-center p-4 sm:p-6">
        <div className="relative z-10 max-h-[90vh] w-full max-w-5xl overflow-hidden rounded-[2rem] border border-white/10 bg-base-100 shadow-[0_32px_90px_-40px_rgba(15,23,42,0.85)]">
          <div className="flex items-start justify-between gap-4 border-b border-base-300/70 bg-gradient-to-r from-base-100 via-base-100 to-base-200/65 px-5 py-5 sm:px-6">
            <div className="flex min-w-0 items-start gap-4">
              <div className="flex size-12 shrink-0 items-center justify-center rounded-2xl border border-base-300/80 bg-base-200 text-base-content/75">
                <Layers className="size-5" />
              </div>
              <div className="min-w-0">
                <h2 className="text-lg font-semibold text-base-content sm:text-xl">
                  {t("assets.stackDetail.title", {
                    defaultValue: "Stack details",
                  })}
                </h2>
                <p className="mt-1 truncate text-sm text-base-content/65">
                  {asset.original_filename ||
                    t("assets.stackDetail.assetFallback", {
                      defaultValue: "Current asset",
                    })}
                </p>
                <p className="mt-2 text-xs uppercase tracking-[0.2em] text-base-content/45">
                  {t("assets.stackDetail.memberCount", {
                    count: memberCount,
                    defaultValue:
                      memberCount === 1 ? "1 asset in stack" : `${memberCount} assets in stack`,
                  })}
                </p>
              </div>
            </div>

            <button
              type="button"
              className="btn btn-ghost btn-sm btn-circle"
              onClick={onClose}
              aria-label={t("assets.stackDetail.close", {
                defaultValue: "Close stack details",
              })}
            >
              <X className="size-4" />
            </button>
          </div>

          <div className="max-h-[calc(90vh-112px)] overflow-y-auto px-5 py-5 sm:px-6 sm:py-6">
            {stackQuery.isPending ? (
              <div className="flex min-h-64 items-center justify-center">
                <div className="inline-flex items-center gap-3 rounded-full border border-base-300/80 bg-base-100 px-4 py-2 text-sm text-base-content/70 shadow-sm">
                  <span className="loading loading-spinner loading-sm"></span>
                  {t("assets.stackDetail.loading", {
                    defaultValue: "Loading stack details...",
                  })}
                </div>
              </div>
            ) : stackQuery.isError ? (
              <div className="alert border border-warning/25 bg-warning/10 text-warning-content">
                <span>
                  {t("assets.stackDetail.error", {
                    defaultValue: "Stack details are temporarily unavailable for this asset.",
                  })}
                </span>
              </div>
            ) : members.length === 0 ? (
              <div className="alert border border-base-300 bg-base-200/70 text-base-content/70">
                <span>
                  {t("assets.stackDetail.empty", {
                    defaultValue: "No related assets were returned for this stack.",
                  })}
                </span>
              </div>
            ) : (
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {members.map((member) => {
                  const position = member.position ?? 0;
                  const memberAssetId = member.primary_asset_id ?? asset.asset_id ?? "";
                  const thumbnailUrl = assetUrls.getThumbnailUrl(memberAssetId, "medium");

                  return (
                    <article key={memberAssetId} className={memberCardClasses(member.isCurrent)}>
                      <div className="relative aspect-[4/3] overflow-hidden bg-base-200">
                        <img
                          src={thumbnailUrl}
                          alt={t("assets.stackDetail.thumbnailAlt", {
                            defaultValue: "Stack member preview",
                          })}
                          className="h-full w-full object-cover"
                          loading="lazy"
                        />

                        <div className="absolute left-3 top-3 flex flex-wrap gap-2">
                          {member.isCover && (
                            <span className="rounded-full border border-black/10 bg-black/60 px-2.5 py-1 text-[11px] font-medium uppercase tracking-wide text-white backdrop-blur-sm">
                              {t("assets.stackDetail.coverBadge", {
                                defaultValue: "Cover",
                              })}
                            </span>
                          )}
                          {member.isCurrent && (
                            <span className="rounded-full border border-primary/20 bg-primary/85 px-2.5 py-1 text-[11px] font-medium uppercase tracking-wide text-primary-content shadow-sm">
                              {t("assets.stackDetail.currentBadge", {
                                defaultValue: "Current",
                              })}
                            </span>
                          )}
                        </div>

                        <div className="absolute bottom-3 right-3 rounded-full border border-white/10 bg-black/60 px-2.5 py-1 text-[11px] font-medium text-white shadow-lg backdrop-blur-sm">
                          #{position + 1}
                        </div>
                      </div>

                      <div className="space-y-3 px-4 py-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="text-sm font-semibold text-base-content">
                              {t("assets.stackDetail.frameLabel", {
                                position: position + 1,
                                defaultValue: "Frame {{position}}",
                              })}
                            </p>
                            <p className="mt-1 font-mono text-xs text-base-content/50">
                              {truncateAssetId(memberAssetId)}
                            </p>
                          </div>
                          <span className="badge badge-ghost shrink-0">#{position + 1}</span>
                        </div>
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

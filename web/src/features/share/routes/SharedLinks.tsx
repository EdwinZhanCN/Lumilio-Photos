import { useMemo, useState } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { Ban, Clock, Eye, Share2, Trash2 } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useShareLinks, type ShareLinkDTO } from "../hooks/useShareLinks";

type StatusFilter = "active" | "expired" | "revoked";

const STATUS_FILTERS: StatusFilter[] = ["active", "expired", "revoked"];

const SOURCE_LABEL_KEYS: Record<string, string> = {
  asset_snapshot: "share.manage.source.assetSnapshot",
  album: "share.manage.source.album",
  person: "share.manage.source.person",
  utility_query: "share.manage.source.utilityQuery",
  pin: "share.manage.source.pin",
};

/** Owner-scoped tokens are stored hash-only server-side (see share_link_service.go),
 * so a link's URL can only ever be copied once, at creation time. There is
 * deliberately no "copy" action here. */
function deriveStatus(link: ShareLinkDTO): StatusFilter {
  if (link.status === "revoked") return "revoked";
  if (link.expires_at && new Date(link.expires_at).getTime() < Date.now()) return "expired";
  return "active";
}

function SharedLinksContent() {
  const { t, i18n } = useI18n();
  const showMessage = useMessage();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.utilities", "Utilities"), to: "/collections/utilities" },
    { label: t("share.manage.pageTitle", "Shared Links") },
  ]);

  const { links, isLoading, revokeShareLink, deleteShareLink, updateShareLink } = useShareLinks();
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("active");
  const [pendingId, setPendingId] = useState<string | null>(null);

  const filtered = useMemo(
    () => links.filter((link) => deriveStatus(link) === statusFilter),
    [links, statusFilter],
  );

  const counts = useMemo(() => {
    const result: Record<StatusFilter, number> = { active: 0, expired: 0, revoked: 0 };
    for (const link of links) result[deriveStatus(link)] += 1;
    return result;
  }, [links]);

  const withPending = async (shareId: string | undefined, run: () => Promise<void>) => {
    if (!shareId) return;
    setPendingId(shareId);
    try {
      await run();
    } finally {
      setPendingId(null);
    }
  };

  const handleRevoke = (link: ShareLinkDTO) =>
    withPending(link.share_id, async () => {
      try {
        await revokeShareLink(link.share_id!);
        showMessage("success", t("share.manage.revokeSuccess", "Share link revoked."));
      } catch (error) {
        console.error("Failed to revoke share link:", error);
        showMessage("error", t("share.manage.revokeError", "Failed to revoke share link."));
      }
    });

  const handleExtend = (link: ShareLinkDTO) =>
    withPending(link.share_id, async () => {
      try {
        await updateShareLink(link.share_id!, { extend_days: 30 });
        showMessage("success", t("share.manage.extendSuccess", "Extended by 30 days."));
      } catch (error) {
        console.error("Failed to extend share link:", error);
        showMessage("error", t("share.manage.extendError", "Failed to extend share link."));
      }
    });

  const handleDelete = (link: ShareLinkDTO) =>
    withPending(link.share_id, async () => {
      try {
        await deleteShareLink(link.share_id!);
        showMessage("success", t("share.manage.deleteSuccess", "Share link deleted."));
      } catch (error) {
        console.error("Failed to delete share link:", error);
        showMessage("error", t("share.manage.deleteError", "Failed to delete share link."));
      }
    });

  const formatDate = (value?: string) =>
    value ? new Date(value).toLocaleDateString(i18n.resolvedLanguage || i18n.language) : "—";

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("share.manage.pageTitle", "Shared Links")}
        icon={<Share2 className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      />

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-4">
          <div className="join">
            {STATUS_FILTERS.map((status) => (
              <button
                key={status}
                type="button"
                className={`btn btn-sm join-item ${statusFilter === status ? "btn-primary" : "btn-ghost"}`}
                onClick={() => setStatusFilter(status)}
              >
                {t(`share.manage.filters.${status}`, status)} ({counts[status]})
              </button>
            ))}
          </div>

          {isLoading && (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, idx) => (
                <div key={idx} className="h-16 animate-pulse rounded-2xl bg-base-200" />
              ))}
            </div>
          )}

          {!isLoading && filtered.length === 0 && (
            <div className="rounded-[1.75rem] border border-dashed border-base-300 px-6 py-12 text-center text-base-content/60">
              <p className="text-base font-semibold">
                {t("share.manage.emptyTitle", "No shared links here")}
              </p>
              <p className="mt-1 text-sm">
                {t("share.manage.emptyDescription", "Share links you create will show up here.")}
              </p>
            </div>
          )}

          {!isLoading && filtered.length > 0 && (
            <div className="overflow-x-auto rounded-2xl border border-base-200">
              <table className="table">
                <thead>
                  <tr>
                    <th>{t("share.manage.columns.title", "Title")}</th>
                    <th>{t("share.manage.columns.source", "Source")}</th>
                    <th>{t("share.manage.columns.assets", "Items")}</th>
                    <th>{t("share.manage.columns.created", "Created")}</th>
                    <th>{t("share.manage.columns.expires", "Expires")}</th>
                    <th>{t("share.manage.columns.lastViewed", "Last viewed")}</th>
                    <th>{t("share.manage.columns.views", "Views")}</th>
                    <th className="text-right">{t("share.manage.columns.actions", "Actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((link) => {
                    const isPending = pendingId === link.share_id;
                    const sourceLabelKey = link.source_kind
                      ? SOURCE_LABEL_KEYS[link.source_kind]
                      : undefined;
                    return (
                      <tr key={link.share_id}>
                        <td className="max-w-56 truncate font-medium">{link.title}</td>
                        <td className="text-sm text-base-content/60">
                          {sourceLabelKey
                            ? t(sourceLabelKey, String(link.source_kind ?? ""))
                            : link.source_kind}
                        </td>
                        <td>{link.asset_count ?? 0}</td>
                        <td className="text-sm text-base-content/60">
                          {formatDate(link.created_at)}
                        </td>
                        <td className="text-sm text-base-content/60">
                          {formatDate(link.expires_at)}
                        </td>
                        <td className="text-sm text-base-content/60">
                          {link.last_viewed_at
                            ? formatDate(link.last_viewed_at)
                            : t("share.manage.neverViewed", "Never")}
                        </td>
                        <td>
                          <span className="inline-flex items-center gap-1 text-sm text-base-content/60">
                            <Eye className="size-3.5" />
                            {link.view_count ?? 0}
                          </span>
                        </td>
                        <td>
                          <div className="flex items-center justify-end gap-1.5">
                            {statusFilter === "active" && (
                              <>
                                <button
                                  type="button"
                                  className="btn btn-ghost btn-xs gap-1"
                                  onClick={() => handleExtend(link)}
                                  disabled={isPending}
                                >
                                  <Clock className="size-3.5" />
                                  {t("share.manage.actions.extend", "Extend")}
                                </button>
                                <button
                                  type="button"
                                  className="btn btn-ghost btn-xs gap-1 text-error"
                                  onClick={() => handleRevoke(link)}
                                  disabled={isPending}
                                >
                                  <Ban className="size-3.5" />
                                  {t("share.manage.actions.revoke", "Revoke")}
                                </button>
                              </>
                            )}
                            {statusFilter !== "active" && (
                              <button
                                type="button"
                                className="btn btn-ghost btn-xs gap-1 text-error"
                                onClick={() => handleDelete(link)}
                                disabled={isPending}
                              >
                                <Trash2 className="size-3.5" />
                                {t("share.manage.actions.delete", "Delete")}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function SharedLinks() {
  const { t } = useI18n();
  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <SharedLinksContent />
    </ErrorBoundary>
  );
}

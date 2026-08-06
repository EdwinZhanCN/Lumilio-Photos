import { AlertTriangle, CheckCircle2, LoaderCircle, ShieldQuestion, XCircle } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useLumilioChatStore } from "../../../state/chatStore";
import type { ConfirmationInfo, ConfirmBlock as ConfirmBlockData } from "../../../model/chatTypes";

interface ConfirmBlockProps {
  block: ConfirmBlockData;
}

const getAction = (info: ConfirmationInfo | undefined) => info?.action ?? info?.Action;
const getCount = (info: ConfirmationInfo | undefined) => info?.count ?? info?.Count;
const getLegacyMessage = (info: ConfirmationInfo | undefined) => info?.message ?? info?.Message;
const getTitle = (info: ConfirmationInfo | undefined) => info?.title ?? info?.Title;

/** Inline confirmation card for a paused consequential action. The card does
 * not claim success until a durable effect receipt arrives from the server. */
export function ConfirmBlock({ block }: ConfirmBlockProps) {
  const { t } = useI18n();
  const confirmInterrupt = useLumilioChatStore((state) => state.confirmInterrupt);

  const rootCause = block.interrupt.InterruptContexts.find((context) => context.IsRootCause);
  if (!rootCause) return null;

  const info = rootCause.Info;
  const action = getAction(info);
  const count = getCount(info);
  const title = getTitle(info);
  const message =
    action === "create_album" && title && typeof count === "number"
      ? t("lumilio.chat.confirmation.createAlbum", { count, title })
      : action === "add_to_album" && typeof count === "number"
        ? t("lumilio.chat.confirmation.addToAlbum", "Add {{count}} photos to this album?", {
            count,
          })
        : getLegacyMessage(info);

  const submitting =
    block.state === "submitting_approval" || block.state === "submitting_rejection";
  const canRespond = block.state === "pending" || block.state === "failed";

  return (
    <div className="my-3 max-w-md rounded-xl border border-warning/30 bg-warning/5 p-4">
      <div className="flex items-center gap-2 text-sm font-medium text-warning">
        <ShieldQuestion size={16} strokeWidth={1.5} />
        {t("lumilio.chat.confirmation.title")}
      </div>
      {message && <p className="my-2 text-sm text-base-content/80">{message}</p>}

      {block.state === "committed" && (
        <div className="mt-2 flex items-start gap-2 text-xs text-success" role="status">
          <CheckCircle2 size={15} className="mt-0.5 shrink-0" />
          <span>{block.receipt?.message ?? t("lumilio.chat.confirmation.committed", "Applied")}</span>
        </div>
      )}
      {block.state === "rejected" && (
        <div className="mt-2 flex items-start gap-2 text-xs text-base-content/55" role="status">
          <XCircle size={15} className="mt-0.5 shrink-0" />
          <span>
            {block.receipt?.message ?? t("lumilio.chat.confirmation.rejected", "Cancelled")}
          </span>
        </div>
      )}
      {block.state === "cancelled" && (
        <div className="mt-2 text-xs text-base-content/50">
          {t("lumilio.messages.stopped", "Stopped")}
        </div>
      )}
      {submitting && (
        <div className="mt-2 flex items-center gap-2 text-xs text-base-content/60" role="status">
          <LoaderCircle size={14} className="animate-spin" />
          {block.state === "submitting_approval"
            ? t("lumilio.chat.confirmation.applying", "Applying after server confirmation…")
            : t("lumilio.chat.confirmation.rejecting", "Cancelling on the server…")}
        </div>
      )}
      {block.state === "failed" && (
        <div className="mt-2 flex items-start gap-2 rounded-lg bg-error/10 p-2 text-xs text-error" role="alert">
          <AlertTriangle size={14} className="mt-0.5 shrink-0" />
          <span>
            {block.error ??
              t("lumilio.chat.confirmation.failed", "The action was not confirmed. Retry safely.")}
          </span>
        </div>
      )}

      {canRespond && (
        <div className="mt-3 flex gap-2">
          <button
            type="button"
            className="rounded-lg bg-success px-3 py-1.5 text-sm font-medium text-success-content transition-all hover:brightness-90"
            onClick={() => void confirmInterrupt(rootCause.ID, true)}
          >
            {block.state === "failed"
              ? t("lumilio.chat.confirmation.retry", "Retry confirm")
              : t("lumilio.chat.confirmation.confirm")}
          </button>
          <button
            type="button"
            className="rounded-lg bg-base-200 px-3 py-1.5 text-sm font-medium text-base-content transition-all hover:bg-base-300"
            onClick={() => void confirmInterrupt(rootCause.ID, false)}
          >
            {t("common.cancel")}
          </button>
        </div>
      )}
    </div>
  );
}

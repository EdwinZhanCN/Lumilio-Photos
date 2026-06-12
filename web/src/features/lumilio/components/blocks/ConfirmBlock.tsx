import { ShieldQuestion } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useLumilioChatStore } from "../../state/chatStore";
import type {
  ConfirmationInfo,
  ConfirmBlock as ConfirmBlockData,
} from "../../types";

interface ConfirmBlockProps {
  block: ConfirmBlockData;
}

const getAction = (info: ConfirmationInfo | undefined) =>
  info?.action ?? info?.Action;

const getCount = (info: ConfirmationInfo | undefined) =>
  info?.count ?? info?.Count;

const getLegacyMessage = (info: ConfirmationInfo | undefined) =>
  info?.message ?? info?.Message;

const getTitle = (info: ConfirmationInfo | undefined) =>
  info?.title ?? info?.Title;

/** Inline confirmation card for an interrupted agent run (preview-then-
 * confirm for consequential actions). Resolving it resumes the stream. */
export function ConfirmBlock({ block }: ConfirmBlockProps) {
  const { t } = useI18n();
  const confirmInterrupt = useLumilioChatStore((s) => s.confirmInterrupt);

  const rootCause = block.interrupt.InterruptContexts.find(
    (ctx) => ctx.IsRootCause,
  );
  if (!rootCause) return null;

  const resolved = block.resolved;
  const info = rootCause.Info;
  const action = getAction(info);
  const count = getCount(info);
  const title = getTitle(info);
  const message =
    action === "create_album" && title && typeof count === "number"
      ? t("lumilio.chat.confirmation.createAlbum", { count, title })
      : getLegacyMessage(info);

  return (
    <div className="my-3 rounded-xl border border-warning/30 bg-warning/5 p-4 max-w-md">
      <div className="flex items-center gap-2 font-medium text-warning text-sm">
        <ShieldQuestion size={16} strokeWidth={1.5} />
        {t("lumilio.chat.confirmation.title")}
      </div>
      {message && <p className="text-sm my-2 text-base-content/80">{message}</p>}

      {resolved ? (
        <div className="text-xs text-base-content/50">
          {resolved === "approved"
            ? t("lumilio.chat.confirmation.approved", "Confirmed")
            : t("lumilio.chat.confirmation.rejected", "Cancelled")}
        </div>
      ) : (
        <div className="flex gap-2 mt-2">
          <button
            className="px-3 py-1.5 text-sm font-medium rounded-lg bg-success text-success-content hover:brightness-90 transition-all"
            onClick={() => void confirmInterrupt(rootCause.ID, true)}
          >
            {t("lumilio.chat.confirmation.confirm")}
          </button>
          <button
            className="px-3 py-1.5 text-sm font-medium rounded-lg bg-base-200 text-base-content hover:bg-base-300 transition-all"
            onClick={() => void confirmInterrupt(rootCause.ID, false)}
          >
            {t("common.cancel")}
          </button>
        </div>
      )}
    </div>
  );
}

import type { ReactNode } from "react";
import { ShareLinkCreateModal } from "@/features/share";
import { useI18n } from "@/lib/i18n";
import type { EventDetail, EventShareRequest } from "../../../model/event";
import { eventTitle } from "../../../model/event";

type EventShareModalProps = {
  open: boolean;
  event: EventDetail;
  isSharing: boolean;
  onClose: () => void;
  onShare: (request: EventShareRequest) => Promise<{ token?: string }>;
};

/** Event-specific snapshot creation backed by the shared share-link UX. */
export default function EventShareModal({
  open,
  event,
  isSharing,
  onClose,
  onShare,
}: EventShareModalProps): ReactNode {
  const { t } = useI18n();

  return (
    <ShareLinkCreateModal
      open={open}
      onClose={onClose}
      defaultTitle={eventTitle(event, t)}
      isCreating={isSharing}
      errorMessage={t("events.shareError", "Failed to share Event.")}
      onCreate={(values) =>
        onShare({
          title: values.title,
          description: values.description,
          expires_in_days: values.expiresInDays,
          allow_download: values.allowDownload,
          include_originals: values.includeOriginals,
        })
      }
    />
  );
}

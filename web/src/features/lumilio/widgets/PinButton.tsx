import { Check, Pin } from "lucide-react";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { getWidget } from "./registry";

interface PinButtonProps {
  refId: string;
  threadId: string;
  widget: string;
  title?: string;
}

/** Pins a chat widget onto the durable board (POST /agent/pins). The pin
 * copies the ref snapshot server-side, so it outlives the conversation. */
export function PinButton({ refId, threadId, widget, title }: PinButtonProps) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [pinned, setPinned] = useState(false);
  const mutation = $api.useMutation("post", "/api/v1/agent/pins", {
    onSuccess: () => {
      setPinned(true);
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/agent/pins"],
      });
    },
  });

  const layout = getWidget(widget)?.defaultLayout;

  if (pinned) {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-success">
        <Check size={13} />
        {t("lumilio.widgets.pinned", "Pinned to board")}
      </span>
    );
  }

  return (
    <button
      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-full border border-base-300 text-base-content/70 hover:text-base-content hover:bg-base-200 transition-all disabled:opacity-50"
      disabled={mutation.isPending}
      onClick={() =>
        mutation.mutate({
          body: {
            ref_id: refId,
            thread_id: threadId,
            title: title ?? "",
            widget,
            layout: layout ? { x: 0, y: 0, w: layout.w, h: layout.h } : undefined,
          },
        })
      }
    >
      <Pin size={13} strokeWidth={1.5} />
      {t("lumilio.widgets.pin", "Pin to board")}
    </button>
  );
}

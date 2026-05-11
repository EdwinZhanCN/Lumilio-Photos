import { BadgeCheck, Ghost, Hammer, Loader, X } from "lucide-react";
import React from "react";
import { SideChannelEvent } from "@/features/lumilio/schema";
import { useLumilioChat } from "@/features/lumilio/hooks/useLumilioChat";
import { AssetGalleryRenderer } from "@/features/lumilio/components/ToolRenderers";
import { useI18n } from "@/lib/i18n.tsx";

export const MarkdownToolBlock: React.FC<any> = ({ id, ...props }) => {
  const { t } = useI18n();
  const { state } = useLumilioChat();

  // 1. Get raw ID (id or data-id)
  const rawId = id || props["data-id"];

  // 2. Remove auto-added "user-content-" prefix
  const executionId = rawId
    ? String(rawId).replace(/^user-content-/, "")
    : null;

  if (!executionId) return null;

  const event = state.conversation
    .flatMap((msg) => msg.sideEvents)
    .find((e) => e.tool.executionId === executionId);

  if (!event) {
    return (
      <div className="text-xs text-error/70 py-2">
        {t("lumilio.tools.eventNotFound")}
      </div>
    );
  }

  switch (event.data?.rendering?.component) {
    case "justified_gallery":
      return (
        <div className="flex flex-wrap items-center gap-2 my-3">
          <LumilioTool event={event} />
          <AssetGalleryRenderer event={event} />
        </div>
      );
    default:
      return (
        <div className="my-3">
          <LumilioTool event={event} />
        </div>
      );
  }
};

const StatusIcon: React.FC<{
  status: "pending" | "running" | "success" | "error" | "cancelled";
}> = ({ status }) => {
  switch (status) {
    case "pending":
    case "running":
      return <Loader className="text-info animate-spin" size={13} />;
    case "success":
      return <BadgeCheck className="text-success" size={13} />;
    case "error":
      return <X className="text-error" size={13} />;
    case "cancelled":
      return <Ghost className="text-base-content/40" size={13} />;
    default:
      return null;
  }
};

export const LumilioTool: React.FC<{ event: SideChannelEvent }> = ({
  event,
}) => {
  const { tool, execution } = event;

  const statusAccent: Record<string, string> = {
    pending: "border-info/30 text-info",
    running: "border-info/30 text-info",
    success: "border-success/30 text-success",
    error: "border-error/30 text-error",
    cancelled: "border-base-content/20 text-base-content/40",
  };

  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-medium transition-colors duration-200 ${statusAccent[execution.status]}`}
    >
      <Hammer className="flex-shrink-0" size={12} strokeWidth={1.5} />
      <span className="truncate max-w-[160px]">{tool.name}</span>
      <StatusIcon status={execution.status} />
      {(execution.message || execution.error) && (
        <span className="text-base-content/50 truncate max-w-[200px] ml-0.5">
          {execution.message && (
            <span className="truncate">{execution.message}</span>
          )}
          {execution.error && (
            <span className="text-error/80 truncate">
              {execution.error.message}
            </span>
          )}
        </span>
      )}
    </span>
  );
};

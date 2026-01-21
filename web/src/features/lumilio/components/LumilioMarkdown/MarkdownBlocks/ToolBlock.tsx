import { BadgeCheck, Ghost, Hammer, X } from "lucide-react";
import React from "react";
import { SideChannelEvent } from "@/features/lumilio/schema";
import { useLumilioChat } from "@/features/lumilio/hooks/useLumilioChat";
import {AssetGalleryRenderer} from "@/features/lumilio/components/ToolRenderers";

export const MarkdownToolBlock: React.FC<any> = ({ id, ...props }) => {
  const { state } = useLumilioChat();

  // 1. 获取原始 ID (id 或 data-id)
  const rawId = id || props["data-id"];

  // 2. 核心修复：移除自动添加的 "user-content-" 前缀
  const executionId = rawId ? String(rawId).replace(/^user-content-/, "") : null;

  if (!executionId) return null;

  const event = state.conversation
    .flatMap((msg) => msg.uiEvents)
    .find((e) => e.tool.executionId === executionId);

  if (!event) {
    // 调试信息：同时显示处理后的 ID 和原始 ID，方便排查
    return (
      <div className="text-xs text-error p-2 border border-error rounded my-2">
        Tool event not found. <br />
        Looked for: <b>{executionId}</b> <br />
        Received raw id: {rawId}
      </div>
    );
  }

  switch (event.data?.rendering?.component) {
    case "justified_gallery":
      return (
        <div className="flex gap-2">
          <LumilioTool event={event} />
          <AssetGalleryRenderer event={event} />
        </div>
      )
    default:
      return <LumilioTool event={event} />;
  };
};

const StatusIcon: React.FC<{
  status: "pending" | "running" | "success" | "error" | "cancelled";
}> = ({ status }) => {
  switch (status) {
    case "pending":
    case "running":
      return <span className="loading loading-spinner loading-xs text-info" />;
    case "success":
      return <BadgeCheck className="text-success" size={15} />;
    case "error":
      return <X className="text-error" size={15} />;
    case "cancelled":
      return <Ghost className="text-base-content/60" size={15} />;
    default:
      return null;
  }
};

export const LumilioTool: React.FC<{ event: SideChannelEvent }> = ({
                                                                     event,
                                                                   }) => {
  const { tool, execution } = event;

  const statusColors: Record<string, string> = {
    pending: "bg-info/10 text-info border-info/20",
    running: "bg-info/10 text-info border-info/20",
    success: "bg-success/10 text-success border-success/20",
    error: "bg-error/10 text-error border-error/20",
    cancelled: "bg-base-200 text-base-content/60 border-base-content/20",
  };

  return (
    <div className="group max-w-max relative">
      <div
        className={`flex items-center gap-1.5 p-1.5 rounded-md border text-xs transition-all duration-150 ${statusColors[execution.status]} hover:shadow-xs`}
      >
        <Hammer className="text-primary flex-shrink-0" size={15} />
        <span className="font-medium truncate flex-1 min-w-0">{tool.name}</span>
        <StatusIcon status={execution.status} />
        {(execution.message || execution.error) && (
          <div className="mx-2 border-base-content/20 text-xs animate-in fade-in slide-in-from-top-1 duration-200">
            {execution.message && (
              <p className="text-base-content/70 py-0.5 truncate">
                {execution.message}
              </p>
            )}
            {execution.error && (
              <p className="text-error/90 py-0.5 truncate">
                {execution.error.message}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

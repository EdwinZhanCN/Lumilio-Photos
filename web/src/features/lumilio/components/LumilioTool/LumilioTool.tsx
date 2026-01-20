import React from "react";
import type { SideChannelEvent } from "../../schema";
import { BadgeCheck, Ghost, Hammer, X } from "lucide-react";

const StatusIcon: React.FC<{
  status: "pending" | "running" | "success" | "error" | "cancelled";
}> = ({ status }) => {
  switch (status) {
    case "pending":
    case "running":
      return <span className="loading loading-spinner loading-xs text-info" />;
    case "success":
      return <BadgeCheck className="text-success" size={14} />;
    case "error":
      return <X className="text-error" size={14} />;
    case "cancelled":
      return <Ghost className="text-primary" size={14} />;
    default:
      return null;
  }
};

export const LumilioTool: React.FC<{ event: SideChannelEvent }> = ({
  event,
}) => {
  const { tool, execution } = event;

  const statusBadgeClasses: Record<string, string> = {
    pending: "badge-info",
    running: "badge-info",
    success: "badge-success",
    error: "badge-error",
    cancelled: "badge-ghost",
  };

  return (
    <div className="flex items-center my-2 text-sm rounded-md bg-base-300">
      <Hammer className="text-primary mx-2" strokeWidth={1.25} />
      <div className="my-2">
        <div className="flex items-center justify-between p-2">
          <div className="flex items-center gap-2">
            <StatusIcon status={execution.status} />
            <span className="font-semibold">{tool.name}</span>
            <span
              className={`badge badge-xs ${statusBadgeClasses[execution.status]}`}
            >
              {execution.status}
            </span>
          </div>
        </div>

        {(execution.message || execution.error) && (
          <div className="px-3 pb-2 text-base-content/80">
            {execution.message && <p>{execution.message}</p>}
            {execution.error && (
              <p className="text-error">{execution.error.message}</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

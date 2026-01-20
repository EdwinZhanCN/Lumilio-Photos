import { BadgeCheck, Ghost, Hammer, X } from "lucide-react";
import { GalleryThumbnails } from "lucide-react";
import { useState } from "react";
import { SideChannelEvent } from "@/features/lumilio/schema";
import type { AssetDTO } from "@/lib/http-commons";
import { assetService } from "@/services";
import { useLumilioChat } from "@/features/lumilio/hooks/useLumilioChat";

/** Renders a tool block by looking up its execution event from the chat state.

  Extracts the execution ID from the provided props, searches the conversation
  history for the corresponding side-channel event, and renders the appropriate
  tool status UI. Handles ID cleanup by removing the "user-content-" prefix
  automatically added by markdown sanitizers.

  Args:
    id: The execution ID of the tool (may include "user-content-" prefix).
    props: Additional props including "data-id" as fallback for ID.

  Returns:
    The DynamicToolBlock component if the event is found, or an error message
    if the tool event cannot be located in the conversation history.
*/
export const MarkdownToolBlock: React.FC<any> = ({ id, ...props }) => {
  const { state } = useLumilioChat();

  // 1. 获取原始 ID (id 或 data-id)
  const rawId = id || props["data-id"];

  // 2. 核心修复：移除自动添加的 "user-content-" 前缀
  const executionId = rawId
    ? String(rawId).replace(/^user-content-/, "")
    : null;

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

  return <DynamicToolBlock event={event} />;
};

/** Renders dynamic tool content including status and optional gallery modal.

  Displays the tool status UI and conditionally renders a gallery button if the
  tool output includes asset data. When the gallery button is clicked, opens a
  modal displaying thumbnails of all assets returned by the tool execution.

  Props:
    event: The side-channel event containing tool execution data, status, and output.

  State:
    isModalOpen: Controls the visibility of the gallery modal dialog.
*/
const DynamicToolBlock: React.FC<{ event: SideChannelEvent }> = ({ event }) => {
  const [isModalOpen, setIsModalOpen] = useState(false);

  const shouldShowGalleryButton =
    event.data?.rendering?.component === "justified_gallery";
  const assets: AssetDTO[] = shouldShowGalleryButton
    ? (event.data?.payload as AssetDTO[])
    : [];

  return (
    <>
      {/* 1. Always render the tool status UI */}
      <LumilioTool event={event} />

      {/* 2. Conditionally render the button to open the modal */}
      {shouldShowGalleryButton && assets.length > 0 && (
        <button
          className="btn my-2 btn-sm btn-outline btn-primary"
          onClick={() => setIsModalOpen(true)}
        >
          <GalleryThumbnails className="h-4 w-4 mr-2" />
          View Gallery ({assets.length})
        </button>
      )}

      {/* 3. The Modal dialog itself, using DaisyUI classes */}
      <div className={`modal ${isModalOpen ? "modal-open" : ""}`}>
        <div className="modal-box w-11/12 max-w-5xl">
          <h3 className="font-bold text-lg">Asset Gallery</h3>
          <p className="py-4 text-sm">{event.execution.message}</p>

          <div className="bg-base-200 rounded-lg p-2 max-h-[60vh] overflow-y-auto">
            <div className="flex flex-wrap gap-2 justify-center">
              {assets.map((asset) => (
                <img
                  key={asset.asset_id}
                  src={assetService.getThumbnailUrl(
                    asset.asset_id ?? "",
                    "small",
                  )}
                  alt={`Asset ${asset.asset_id}`}
                  className="h-28 w-28 object-cover rounded-md shadow-md hover:scale-105 transition-transform"
                  loading="lazy"
                />
              ))}
            </div>
          </div>

          <div className="modal-action">
            <button className="btn" onClick={() => setIsModalOpen(false)}>
              Close
            </button>
          </div>
        </div>
        <div
          className="modal-backdrop"
          onClick={() => setIsModalOpen(false)}
        ></div>
      </div>
    </>
  );
};

/** Renders an appropriate status icon based on the tool execution status.

  Maps execution statuses to visual indicators: spinner for pending/running,
  checkmark for success, X for error, and ghost icon for cancelled operations.

  Args:
    status: The execution status of the tool.

  Returns:
    A React element containing the appropriate icon for the status.
*/
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

/** Renders the tool status display UI with color-coded styling.

  Displays a compact status badge showing the tool name, execution status icon,
  and optional message or error details. The background and text colors are
  dynamically assigned based on the execution status for visual feedback.

  Props:
    event: The side-channel event containing tool and execution information.

  Returns:
    A styled div component displaying the tool's current execution status.
*/
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
    <div className="group mt-2 max-w-max relative">
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

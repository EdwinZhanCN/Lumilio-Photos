import { useRef, useEffect, useState } from "react";
import { Markdown } from "../LumilioMarkdown/Markdown";
import { LumilioTool } from "../LumilioTool";
import { LumilioAvatar } from "../LumilioAvatar/LumilioAvatar";
import type { ChatMessage } from "@/features/lumilio/lumilio.types";
import { GalleryThumbnails } from "lucide-react";

import { SideChannelEvent } from "../../schema";
import type { AssetDTO } from "@/lib/http-commons";
import { assetService } from "@/services";

const DynamicUIComponent: React.FC<{ event: SideChannelEvent }> = ({
  event,
}) => {
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
        <div className="mt-2">
          <button
            className="btn btn-sm btn-outline btn-primary"
            onClick={() => setIsModalOpen(true)}
          >
            <GalleryThumbnails className="h-4 w-4 mr-2" />
            View Gallery ({assets.length})
          </button>
        </div>
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

function processThinkTags(
  content: string,
  isStreaming: boolean = false,
): string {
  const openTags = (content.match(/<think>/g) || []).length;
  const closeTags = (content.match(/<\/think>/g) || []).length;
  const isCurrentlyThinking = openTags > closeTags;

  let processed = content;
  if (isCurrentlyThinking && isStreaming) {
    let openTagsReplaced = 0;
    processed = processed.replace(/<think>/g, () => {
      openTagsReplaced++;
      return openTagsReplaced === openTags
        ? "<details open><summary> Thinking...</summary>"
        : "<details><summary> Thinking...</summary>";
    });
  } else {
    processed = processed.replace(
      /<think>/g,
      "<details><summary> Thinking...</summary>",
    );
  }
  processed = processed.replace(/<\/think>/g, "</details>");
  return processed;
}

interface LumilioMessagesProps {
  conversation: ChatMessage[];
  isGenerating: boolean;
}

export function LumilioMessages({
  conversation,
  isGenerating,
}: LumilioMessagesProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [conversation]);

  return (
    <div className="p-4 space-y-4">
      {conversation.map((message, index) => {
        const isLast = index === conversation.length - 1;
        const isStreamingHere =
          message.role === "assistant" && isGenerating && isLast;

        return (
          <div
            key={message.id}
            className={`chat ${message.role === "user" ? "chat-end" : "chat-start"}`}
          >
            <div className="">
              <div className="w-10 rounded-full">
                {message.role === "assistant" && (
                  <div className="w-full h-full flex items-center">
                    <LumilioAvatar start={isStreamingHere} size={0.2} />
                  </div>
                )}
              </div>
            </div>
            {/*<div className="chat-header">
              {message.role === "user" ? "You" : "Lumilio"}
            </div>*/}
            <div
              className={` ${
                message.role === "user"
                  ? "chat-bubble chat-bubble-primary"
                  : "rounded-2xl bg-base-200 w-full fadeIn"
              }`}
            >
              {message.content && (
                <Markdown
                  content={processThinkTags(message.content, isStreamingHere)}
                  className={`${message.role === "user" ? "" : "mx-6 my-4"}`}
                />
              )}
              {message.uiEvents.map((uiEvent) => (
                <DynamicUIComponent
                  key={uiEvent.tool.executionId}
                  event={uiEvent}
                />
              ))}
            </div>
          </div>
        );
      })}

      {isGenerating &&
        conversation.length > 0 &&
        conversation[conversation.length - 1]?.role === "user" && (
          <div className="chat chat-start">
            <div className="chat-image avatar">
              <div className="w-10 rounded-full flex items-center justify-center">
                <LumilioAvatar start={true} size={0.1} />
              </div>
            </div>
            <div className="chat-bubble bg-base-200">
              <span className="loading loading-dots loading-md"></span>
            </div>
          </div>
        )}

      <div ref={messagesEndRef} />
    </div>
  );
}

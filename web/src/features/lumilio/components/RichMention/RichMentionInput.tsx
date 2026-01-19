import React from "react";

import { useRichMention } from "./hooks/useRichMention";
import { MentionMenu } from "./MentionMenu";
import { MentionType, ChatMessage } from "./types";

interface RichMentionInputProps {
  onSendMessage: (message: ChatMessage) => void;
  isGenerating: boolean;
  disabled?: boolean;
  className?: string;
}

export function RichMentionInput({
  onSendMessage,
  isGenerating,
  disabled = false,
  className = "",
}: RichMentionInputProps) {
  const {
    editorRef,
    phase,
    menuPos,
    selectedIndex,
    options,
    payload,
    handleInput,
    handleKeyDown,
    handleSubmit,
    clearContent,
    setSelectedIndex,
    insertPill,
    setActiveMentionType,
    setPhase,
    activeMentionType,
  } = useRichMention();

  // 处理选项选择
  const handleSelectOption = (option: any) => {
    if (phase === "SELECT_TYPE") {
      setActiveMentionType(option.type as MentionType);
      setPhase("SELECT_ENTITY");
    } else {
      insertPill(option);
    }
  };

  // 处理发送消息
  const handleSend = async () => {
    if (payload.trim() && !isGenerating) {
      const response = await handleSubmit();
      const chatMessage: ChatMessage = {
        id: `msg-${Date.now()}`,
        role: "user",
        content: payload,
        timestamp: Date.now(),
        command: response?.command,
        commandPayload: response,
      };
      onSendMessage(chatMessage);
      clearContent();
    }
  };

  // 处理键盘事件
  const handleKeyDownEvent = (e: React.KeyboardEvent) => {
    // 如果不是在 mention 模式下，处理 Enter 键发送消息
    if (phase === "IDLE" && e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
      return;
    }

    // 否则交给 hook 处理
    handleKeyDown(e);
  };

  // 处理输入事件
  const handleInputEvent = () => {
    handleInput();
  };

  return (
    <div className={`relative ${className}`}>
      <div className="p-4 bg-base-100 border-t border-base-300 relative">
        <div className="relative bg-base-200 rounded-xl border border-base-300 focus-within:ring-2 focus-within:ring-primary focus-within:bg-base-100 transition-all">
          <div
            ref={editorRef}
            contentEditable
            className="w-full max-h-32 overflow-y-auto p-3 focus:outline-none text-sm leading-relaxed empty:before:content-[attr(data-placeholder)] empty:before:text-base-content/50"
            data-placeholder={
              isGenerating
                ? "Waiting for response..."
                : "Ask Lumilio... (Type / for commands, @ for mentions)"
            }
            onInput={handleInputEvent}
            onKeyDown={handleKeyDownEvent}
            style={{
              pointerEvents: isGenerating || disabled ? "none" : "auto",
            }}
            suppressContentEditableWarning={true}
          />
          <div className="flex justify-between items-center px-2 pb-2 mt-1">
            <div className="flex gap-2 text-xs text-base-content/50">
              {/* Temporarily disabled @ mention functionality
              <span className="flex items-center gap-1 bg-base-100 border rounded px-1.5 py-0.5">
                @ Mention
              </span>
              */}
              <span className="flex items-center gap-1 bg-base-100 border rounded px-1.5 py-0.5">
                / Command
              </span>
            </div>
            <button
              className={`p-2 rounded-lg transition-all duration-200 flex items-center justify-center ${
                payload.trim() && !isGenerating && !disabled
                  ? "bg-linear-to-r from-blue-500 to-purple-600 text-white hover:shadow-lg hover:scale-105"
                  : "bg-base-300 text-base-content/40"
              }`}
              onClick={handleSend}
              disabled={!payload.trim() || isGenerating || disabled}
            >
              {isGenerating ? (
                <span className="loading loading-spinner loading-sm text-primary"></span>
              ) : (
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <line x1="22" y1="2" x2="11" y2="13"></line>
                  <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
                </svg>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Mention Menu */}
      {phase !== "IDLE" && (
        <MentionMenu
          phase={phase}
          activeMentionType={activeMentionType}
          menuPos={menuPos}
          options={options}
          selectedIndex={selectedIndex}
          setSelectedIndex={setSelectedIndex}
          onSelectOption={handleSelectOption}
          editorRef={editorRef}
        />
      )}

      {/* Debug Payload View - Only in development */}
      {process.env.NODE_ENV === "development" && (
        <div className="mt-2 text-xs text-base-content/30 font-mono truncate px-1">
          Payload: {payload}
        </div>
      )}
    </div>
  );
}

export default RichMentionInput;

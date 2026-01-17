import { useState, useRef, useEffect } from "react";

interface LumenInputProps {
  inputMessage: string;
  isGenerating: boolean;
  isInitializing: boolean;
  onInputChange: (value: string) => void;
  onSendMessage: () => void;
  onKeyPress: (e: React.KeyboardEvent) => void;
}

export function LumenInput({
  inputMessage,
  isGenerating,
  isInitializing,
  onInputChange,
  onSendMessage,
  onKeyPress,
}: LumenInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [isFocused, setIsFocused] = useState(false);

  // Auto-resize textarea based on content
  useEffect(() => {
    const textarea = textareaRef.current;
    if (textarea) {
      textarea.style.height = "auto";
      textarea.style.height = `${Math.min(textarea.scrollHeight, 120)}px`;
    }
  }, [inputMessage]);

  // Get status message and color
  const getStatusInfo = () => {
    if (isInitializing)
      return { message: "Initializing model...", color: "text-warning" };
    if (isGenerating)
      return { message: "Generating response...", color: "text-info" };
    return { message: "Ready to assist", color: "text-success" };
  };

  const statusInfo = getStatusInfo();

  return (
    <div className="p-4 border-t border-base-300 bg-base-100/80 backdrop-blur-sm">
      <div
        className={`max-w-3xl mx-auto transition-all duration-200 ${isFocused ? "scale-[1.01]" : ""}`}
      >
        <div className="flex gap-2 items-end bg-base-200 rounded-2xl p-2 shadow-sm border border-base-300 focus-within:border-primary focus-within:shadow-md transition-all duration-200">
          <div className="flex-1">
            <textarea
              ref={textareaRef}
              className="w-full bg-transparent resize-none outline-none text-base-content placeholder:text-base-content/50 px-3 py-2 min-h-[24px] max-h-[120px] rounded-xl"
              placeholder={
                isGenerating
                  ? "Waiting for response..."
                  : "Type your message..."
              }
              value={inputMessage}
              onChange={(e) => onInputChange(e.target.value)}
              onKeyPress={onKeyPress}
              disabled={isGenerating || isInitializing}
              onFocus={() => setIsFocused(true)}
              onBlur={() => setIsFocused(false)}
              rows={1}
            />
          </div>
          <button
            className={`p-2 rounded-xl transition-all duration-200 flex items-center justify-center ${
              inputMessage.trim() && !isGenerating && !isInitializing
                ? "bg-gradient-to-r from-blue-500 to-purple-600 text-white hover:shadow-lg hover:scale-105"
                : "bg-base-300 text-base-content/40"
            }`}
            onClick={onSendMessage}
            disabled={!inputMessage.trim() || isGenerating || isInitializing}
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

        {/* Enhanced status info */}
        <div className="mt-2 flex items-center justify-between">
          <div
            className={`text-xs ${statusInfo.color} flex items-center gap-1`}
          >
            <span
              className={`w-1.5 h-1.5 rounded-full ${isGenerating ? "animate-pulse" : ""} bg-current`}
            ></span>
            {statusInfo.message}
          </div>
          <div className="text-xs text-base-content/50">
            Press{" "}
            <kbd className="px-1.5 py-0.5 bg-base-200 rounded text-xs">
              Enter
            </kbd>{" "}
            to send,{" "}
            <kbd className="px-1.5 py-0.5 bg-base-200 rounded text-xs">
              Shift+Enter
            </kbd>{" "}
            for new line
          </div>
        </div>
      </div>
    </div>
  );
}

export default LumenInput;

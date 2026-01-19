import { useState, useRef, useEffect } from "react";
import { RichMentionInput } from "../RichMention";
import { ChatMessage } from "../RichMention/types";
import { useRuntime } from "../../runtime/AgentRuntimeProvider";

interface LumenRichInputProps {
  isGenerating?: boolean;
  isInitializing?: boolean;
  className?: string;
  onMessageReceived?: (message: ChatMessage) => void;
}

export function LumenRichInput({
  isGenerating = false,
  isInitializing = false,
  className = "",
  onMessageReceived,
}: LumenRichInputProps) {
  const { append } = useRuntime();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [localMessages, setLocalMessages] = useState<ChatMessage[]>([]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [localMessages]);

  const handleSendMessage = async (
    payload: string,
    response?: { text: string; command?: any },
  ) => {
    // 1. User Message
    const userMsg: ChatMessage = {
      id: `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      role: "user",
      content: payload,
      timestamp: Date.now(),
    };

    // Add to local messages
    setLocalMessages((prev) => [...prev, userMsg]);

    // Send to runtime
    if (append) {
      await append({
        role: "user",
        content: [{ type: "text", text: payload }],
      });
    }

    // If we have a simulated response, add it
    if (response) {
      const assistantMsg: ChatMessage = {
        id: `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        role: "assistant",
        content: response.text,
        timestamp: Date.now(),
        commandPayload: response.command,
      };

      setLocalMessages((prev) => [...prev, assistantMsg]);

      // Notify parent component
      if (onMessageReceived) {
        onMessageReceived(assistantMsg);
      }
    }
  };

  return (
    <div className={`relative ${className}`}>
      <RichMentionInput
        onSendMessage={async (message: ChatMessage) => {
          // Extract the text content from the message
          const textContent = message.content;
          await handleSendMessage(
            textContent,
            message.commandPayload
              ? { text: textContent, command: message.commandPayload }
              : undefined,
          );
        }}
        isGenerating={isGenerating}
        disabled={isInitializing}
      />
    </div>
  );
}

export default LumenRichInput;

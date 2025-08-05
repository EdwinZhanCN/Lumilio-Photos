import React, { useState, useEffect } from "react";
import { useLLM, type LLMProgress } from "@/hooks/util-hooks/useLLM.tsx";
import { useSettings } from "@/contexts/SettingsContext";
import { LumenHeader } from "./LumenHeader";
import { LumenStatus } from "./LumenStatus";
import { LumenMessages } from "./LumenMessages";
import { LumenInput } from "./LumenInput";

export function LumenChat() {
  const { settings } = useSettings();
  const {
    isInitializing,
    isGenerating,
    conversation,
    currentModelId,
    generateAnswer,
    clearConversation,
    cancelGeneration,
    setSystemPrompt,
    progress,
  } = useLLM();

  const [inputMessage, setInputMessage] = useState("");
  const [isNoThink, setNoThink] = useState(false);

  // Set system prompt from settings on component mount
  useEffect(() => {
    setSystemPrompt(settings.lumen?.systemPrompt || "");
  }, [setSystemPrompt, settings.lumen?.systemPrompt]);

  const handleSendMessage = async () => {
    if (!inputMessage.trim() || isGenerating) return;

    const message = inputMessage.trim() + (isNoThink ? "/no_think" : "");
    setInputMessage("");
    await generateAnswer(message);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  return (
    <div className="flex flex-col h-full bg-base-100 rounded-lg shadow-lg">
      <LumenHeader
        currentModelId={currentModelId}
        isInitializing={isInitializing}
        isGenerating={isGenerating}
        conversationLength={conversation.length}
        isNoThink={isNoThink}
        onClearConversation={clearConversation}
        onCancelGeneration={cancelGeneration}
        onToggleThink={setNoThink}
      />

      <LumenStatus
        isInitializing={isInitializing}
        progress={progress as LLMProgress | null}
      />

      <LumenMessages
        conversation={conversation}
        isGenerating={isGenerating}
        isInitializing={isInitializing}
      />

      <LumenInput
        inputMessage={inputMessage}
        isGenerating={isGenerating}
        isInitializing={isInitializing}
        onInputChange={setInputMessage}
        onSendMessage={handleSendMessage}
        onKeyPress={handleKeyPress}
      />
    </div>
  );
}

import React, { useState, useEffect } from "react";
import { useLLM, type LLMProgress } from "@/hooks/util-hooks/useLLM.tsx";
import { LumenHeader } from "./LumenHeader";
import { LumenStatus } from "./LumenStatus";
import { LumenMessages } from "./LumenMessages";
import { LumenInput } from "./LumenInput";

export function LumenChat() {
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
  const [selectedModel, setSelectedModel] = useState("Qwen3-4B-q4f16_1-MLC");
  const [isNoThink, setNoThink] = useState(false);

  // Available models https://mlc.ai/models
  // model list avaliable at https://github.com/mlc-ai/web-llm/blob/main/src/config.ts
  const availableModels = [
    { id: "Qwen3-4B-q4f16_1-MLC", name: "Qwen3-4B" },
    { id: "Qwen3-1.7B-q4f16_1-MLC", name: "Qwen3-1.7B" },
  ];

  // Set system prompt on component mount
  useEffect(() => {
    setSystemPrompt(
      "You are a helpful AI assistant that provides informative and concise responses about various topics. Be friendly and engaging in your responses.",
    );
  }, [setSystemPrompt]);

  const handleModelChange = (modelId: string) => {
    if (isInitializing || isGenerating) return;
    setSelectedModel(modelId);
    clearConversation();
  };

  const handleSendMessage = async () => {
    if (!inputMessage.trim() || isGenerating) return;

    const message = inputMessage.trim() + (isNoThink ? "/no_think" : "");
    setInputMessage("");
    await generateAnswer(message, {
      modelId: selectedModel,
      temperature: 0.7,
    });
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
        selectedModel={selectedModel}
        availableModels={availableModels}
        isInitializing={isInitializing}
        isGenerating={isGenerating}
        conversationLength={conversation.length}
        isNoThink={isNoThink}
        onModelChange={handleModelChange}
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

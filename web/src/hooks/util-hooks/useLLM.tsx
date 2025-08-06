import {useCallback, useEffect, useRef, useState} from "react";
import {useMessage} from "@/hooks/util-hooks/useMessage.tsx";
import {useWorker} from "@/contexts/WorkerProvider.tsx";
import {ChatCompletionMessageParam, InitProgressReport,} from "@mlc-ai/web-llm";
import {useSettingsContext} from "@/features/settings";

export interface LLMProgress {
  isInitializing: boolean;
  isGenerating: boolean;
  tokensGenerated: number;
  initStatus?: string;
  initTime?: number;
  initProgress?: number;
  error?: string;
  failedAt?: number | null;
}

export interface LLMMessage {
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: number;
}

export interface LLMOptions {
  modelId?: string;
  temperature?: number;
  top_p?: number;
  systemPrompt?: string;
  stream?: boolean;
}

export interface UseLLMReturn {
  isInitializing: boolean;
  isGenerating: boolean;
  progress: LLMProgress | null;
  conversation: LLMMessage[];
  currentModelId: string | null;
  generateAnswer: (
    userInput: string,
    options?: LLMOptions,
  ) => Promise<string | undefined>;
  clearConversation: () => void;
  cancelGeneration: () => void;
  setSystemPrompt: (prompt: string) => void;
}

/**
 * Custom hook for LLM interactions using the shared web worker client.
 * It manages conversation state, streaming responses, and progress tracking.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 * @author Edwin Zhan
 * @since 1.1.0
 * @returns {UseLLMReturn} Hook state and actions for LLM interaction.
 */
export const useLLM = (): UseLLMReturn => {
  const showMessage = useMessage();
  const workerClient = useWorker();
  const { state} = useSettingsContext();

  const [isInitializing, setIsInitializing] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [progress, setProgress] = useState<LLMProgress | null>(null);
  const [conversation, setConversation] = useState<LLMMessage[]>([]);
  const [systemPrompt, setSystemPromptState] = useState<string>("");
  const [currentModelId, setCurrentModelId] = useState<string | null>(null);

  // Kick off model initialization on mount so progress shows immediately
  useEffect(() => {
    if (state.lumen?.enabled) {
      setIsInitializing(true);
      workerClient
        .initializeWebLLMEngine(state.lumen.model)
        .catch((err) => {
          showMessage("error", `Model initialization failed: ${err.message}`);
        })
        .finally(() => {
          setIsInitializing(false);
        });
    }
  }, [
    workerClient,
    showMessage,
    state.lumen?.model,
    state.lumen?.enabled,
  ]);

  const abortControllerRef = useRef<AbortController | null>(null);

  // Effect to listen for progress updates from the worker
  useEffect(() => {
    return workerClient.addProgressListener(
        (report: InitProgressReport) => {
          const initializing = report.progress < 1;
          setProgress((prev) => ({
            ...prev!,
            isInitializing: initializing,
            initStatus: report.text,
            initTime: report.timeElapsed,
            initProgress: report.progress,
          }));
        },
    );
  }, [workerClient]);

  const generateAnswer = useCallback(
    async (
      userInput: string,
      options: LLMOptions = {},
    ): Promise<string | undefined> => {
      if (!userInput.trim()) {
        showMessage("error", "Please provide a valid input");
        return undefined;
      }

      if (!state.lumen?.enabled) {
        showMessage("error", "Lumen is disabled in settings");
        return undefined;
      }

      const modelId = options.modelId || state.lumen?.model;

      if (!modelId) {
        showMessage("error", "Please specify a modelId in options or settings");
        return undefined;
      }

      // Update current model if different
      if (currentModelId !== modelId) {
        setCurrentModelId(modelId);
        setIsInitializing(true);
      }

      setIsGenerating(true);
      setProgress({
        isInitializing: false,
        isGenerating: true,
        tokensGenerated: 0,
      });

      abortControllerRef.current = new AbortController();

      // Add user message to conversation
      const userMessage: LLMMessage = {
        role: "user",
        content: userInput,
        timestamp: Date.now(),
      };

      setConversation((prev) => [...prev, userMessage]);

      setConversation((prev) => [
        ...prev,
        { role: "assistant", content: "", timestamp: Date.now() },
      ]);

      try {
        // Prepare messages for the LLM
        const messages: ChatCompletionMessageParam[] = [];

        // Add system prompt if provided
        const currentSystemPrompt = options.systemPrompt || systemPrompt;
        if (currentSystemPrompt) {
          messages.push({
            role: "system",
            content: currentSystemPrompt,
          });
        }

        // Add conversation history
        conversation.forEach((msg) => {
          messages.push({
            role: msg.role,
            content: msg.content,
          });
        });

        // Add current user input
        messages.push({
          role: "user",
          content: userInput,
        });

        const fullResponse = "";
        let tokenCount = 0;

        const response = await workerClient.askLLM(messages, {
          temperature: options.temperature ?? state.lumen?.temperature,
          stream: options.stream ?? true,
          onChunk: (chunk: string) => {
            tokenCount++;
            setConversation((prev) => {
              const copy = [...prev];
              const last = copy[copy.length - 1];
              copy[copy.length - 1] = {
                ...last,
                content: last.content + chunk,
              };
              return copy;
            });
            setProgress((prev) => ({ ...prev!, tokensGenerated: tokenCount }));
          },
        });

        showMessage("success", "Response generated successfully");

        return response || fullResponse;
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : String(error || "Unknown error");

        showMessage("error", `LLM generation failed: ${errorMessage}`);
        setProgress((prev) =>
          prev
            ? {
                ...prev,
                error: errorMessage,
                failedAt: Date.now(),
              }
            : null,
        );

        // Remove the user message if generation failed
        setConversation((prev) => prev.slice(0, -1));

        return undefined;
      } finally {
        setIsGenerating(false);
        setIsInitializing(false);
        abortControllerRef.current = null;
        // Keep progress for a bit to show final state
        setTimeout(() => {
          setProgress(null);
        }, 3000);
      }
    },
    [
      workerClient,
      showMessage,
      systemPrompt,
      currentModelId,
      state.lumen,
      conversation,
    ],
  );

  const clearConversation = useCallback(() => {
    setConversation([]);
    setProgress(null);
    showMessage("info", "Conversation cleared");
  }, [showMessage]);

  const cancelGeneration = useCallback(() => {
    abortControllerRef.current?.abort();
    setIsGenerating(false);
    setIsInitializing(false);
    setProgress(null);
    showMessage("info", "LLM generation cancelled");
  }, [showMessage]);

  const setSystemPrompt = useCallback((prompt: string) => {
    setSystemPromptState(prompt);
  }, []);

  return {
    isInitializing,
    isGenerating,
    progress,
    conversation,
    currentModelId,
    generateAnswer,
    clearConversation,
    cancelGeneration,
    setSystemPrompt,
  };
};

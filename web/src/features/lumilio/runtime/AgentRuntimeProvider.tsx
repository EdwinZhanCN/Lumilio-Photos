/**
 * Agent Runtime Provider
 * Custom implementation that integrates with our backend agent service
 * Replaces assistant-ui dependencies with a minimal custom implementation
 */

"use client";

import type { ReactNode } from "react";
import {
  createContext,
  useContext,
  useState,
  useCallback,
  useRef,
} from "react";

/**
 * Message types for our chat system
 */
export interface Message {
  id: string;
  role: "user" | "assistant" | "system";
  content: MessageContent[];
  createdAt: Date;
}

export interface MessageContent {
  type: "text";
  text: string;
}

/**
 * Runtime state and actions
 */
export interface RuntimeState {
  messages: Message[];
  isRunning: boolean;
  error: string | null;
  commandData?: any; // Store the latest command data from side channel
}

export interface RuntimeActions {
  append: (message: Omit<Message, "id" | "createdAt">) => Promise<void>;
  stop: () => void;
  clear: () => void;
}

export type Runtime = RuntimeState & RuntimeActions;

/**
 * Context for providing the runtime
 */
const RuntimeContext = createContext<Runtime | null>(null);

/**
 * Custom Model Adapter that integrates with our backend agent service
 */
import { agentService, type AgentEvent } from "@/services/agentService";

/**
 * Type guard to check if data is an AgentEvent
 */
function isAgentEvent(data: unknown): data is AgentEvent {
  return (
    typeof data === "object" &&
    data !== null &&
    ("agent_name" in data ||
      "output" in data ||
      "reasoning" in data ||
      "action" in data ||
      "run_path" in data ||
      "error" in data)
  );
}

/**
 * Use Runtime Hook
 */
export const useRuntime = (): Runtime => {
  const context = useContext(RuntimeContext);
  if (!context) {
    throw new Error("useRuntime must be used within an AgentRuntimeProvider");
  }
  return context;
};

/**
 * Runtime Provider Component
 */
export function AgentRuntimeProvider({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  const [state, setState] = useState<RuntimeState>({
    messages: [],
    isRunning: false,
    error: null,
    commandData: null,
  });

  const abortControllerRef = useRef<AbortController | null>(null);

  const append = useCallback(
    async (message: Omit<Message, "id" | "createdAt">) => {
      // Generate a unique ID for the message
      const id = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      const newMessage: Message = {
        ...message,
        id,
        createdAt: new Date(),
      };

      // Add the user message to the conversation
      setState((prevState) => ({
        ...prevState,
        messages: [...prevState.messages, newMessage],
        isRunning: true,
        error: null,
      }));

      // Create a new abort controller for this request
      abortControllerRef.current = new AbortController();

      try {
        // Extract the last user message text
        const textContent = message.content
          .filter((part) => part.type === "text")
          .map((part) => part.text)
          .join("\n");

        // Check if the message contains a command (starts with /)
        const toolNames = [];
        if (textContent.startsWith("/")) {
          const commandMatch = textContent.match(/^\/(\w+)/);
          if (commandMatch) {
            toolNames.push(commandMatch[1]);
          }
        }

        // Create an assistant message to stream the response into
        const assistantId = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        let assistantMessage: Message = {
          id: assistantId,
          role: "assistant",
          content: [{ type: "text", text: "" }],
          createdAt: new Date(),
        };

        // Add the empty assistant message to the conversation
        setState((prevState) => ({
          ...prevState,
          messages: [...prevState.messages, assistantMessage],
        }));

        // Use our agent service to stream the response
        let accumulatedContent = "";
        let reasoningContent = "";
        let showReasoning = false;

        for await (const event of agentService.streamAgentChat(
          { query: textContent, tool_names: toolNames },
          abortControllerRef.current.signal,
        )) {
          if (event.type === "message") {
            const data = event.data;
            // Ensure data is an AgentEvent
            if (isAgentEvent(data)) {
              // Handle reasoning content (思维链)
              if (data.reasoning) {
                reasoningContent += data.reasoning;
                showReasoning = true;

                // Create combined content with reasoning using think tags
                // that LumenMessages.processThinkTags will convert to details elements
                let combinedContent = "";
                if (showReasoning && reasoningContent) {
                  combinedContent += `<think>${reasoningContent}</think>\n\n`;
                }
                combinedContent += accumulatedContent;

                // Update the assistant message
                assistantMessage = {
                  ...assistantMessage,
                  content: [{ type: "text", text: combinedContent }],
                };

                setState((prevState) => ({
                  ...prevState,
                  messages: [
                    ...prevState.messages.slice(0, -1),
                    assistantMessage,
                  ],
                }));
                continue;
              }

              // Handle tool calls (action field)
              if (data.action) {
                const toolName = data.action.name;
                const toolInput = data.action.input;

                // Create tool call visualization
                let toolCallText = `🔧 **调用工具**: \`${toolName}\``;
                if (toolInput && typeof toolInput === "object") {
                  toolCallText += `\n\`\`\`json\n${JSON.stringify(toolInput, null, 2)}\n\`\`\``;
                }

                // Yield tool call as part of accumulated content
                accumulatedContent +=
                  (accumulatedContent ? "\n\n" : "") + toolCallText;

                // Create combined content with reasoning using think tags
                let combinedContent = "";
                if (showReasoning && reasoningContent) {
                  combinedContent += `<think>${reasoningContent}</think>\n\n`;
                }
                combinedContent += accumulatedContent;

                // Update the assistant message
                assistantMessage = {
                  ...assistantMessage,
                  content: [{ type: "text", text: combinedContent }],
                };

                setState((prevState) => ({
                  ...prevState,
                  messages: [
                    ...prevState.messages.slice(0, -1),
                    assistantMessage,
                  ],
                }));
                continue;
              }

              // Handle tool results (output as object)
              if (data.output && typeof data.output === "object") {
                const toolResult = data.output;
                let resultText = "";

                if (Array.isArray(toolResult)) {
                  const toolResultArray = toolResult as unknown[];
                  resultText = `✅ **工具返回 ${toolResultArray.length} 条结果**\n`;
                  if (toolResultArray.length > 0) {
                    resultText += `\`\`\`json\n${JSON.stringify(toolResultArray.slice(0, 3), null, 2)}`;
                    if (toolResultArray.length > 3) {
                      resultText += `\n... 还有 ${toolResultArray.length - 3} 条结果`;
                    }
                    resultText += `\n\`\`\``;
                  }
                } else {
                  resultText = `✅ **工具执行完成**\n\`\`\`json\n${JSON.stringify(toolResult, null, 2)}\n\`\`\``;
                }

                // Yield tool result as part of accumulated content
                accumulatedContent +=
                  (accumulatedContent ? "\n\n" : "") + resultText;

                // Create combined content with reasoning using think tags
                let combinedContent = "";
                if (showReasoning && reasoningContent) {
                  combinedContent += `<think>${reasoningContent}</think>\n\n`;
                }
                combinedContent += accumulatedContent;

                // Update the assistant message
                assistantMessage = {
                  ...assistantMessage,
                  content: [{ type: "text", text: combinedContent }],
                };

                setState((prevState) => ({
                  ...prevState,
                  messages: [
                    ...prevState.messages.slice(0, -1),
                    assistantMessage,
                  ],
                }));
                continue;
              }

              // Handle regular text output
              if (data.output && typeof data.output === "string") {
                const outputText = data.output;

                // Regular text output
                if (outputText && outputText.trim()) {
                  accumulatedContent +=
                    (accumulatedContent ? "" : "") + outputText;

                  // Create combined content with reasoning using think tags
                  let combinedContent = "";
                  if (showReasoning && reasoningContent) {
                    combinedContent += `<think>${reasoningContent}</think>\n\n`;
                  }
                  combinedContent += accumulatedContent;

                  // Update the assistant message
                  assistantMessage = {
                    ...assistantMessage,
                    content: [{ type: "text", text: combinedContent }],
                  };

                  setState((prevState) => ({
                    ...prevState,
                    messages: [
                      ...prevState.messages.slice(0, -1),
                      assistantMessage,
                    ],
                  }));
                }
              }
            }
          } else if (event.type === "ui_event") {
            // Handle UI event from side channel (filter_view, etc.)
            const uiEventData = event.data as any;

            // Update assistant message to indicate UI update
            let uiText = "🖼️ **视图更新**: " + (uiEventData.type || "未知视图");
            if (
              uiEventData.params &&
              Object.keys(uiEventData.params).length > 0
            ) {
              uiText += `\n\`\`\`json\n${JSON.stringify(uiEventData.params, null, 2)}\n\`\`\``;
            }

            // Add UI visualization to the accumulated content
            accumulatedContent += (accumulatedContent ? "\n\n" : "") + uiText;

            // Create combined content with reasoning using think tags
            let combinedContent = "";
            if (showReasoning && reasoningContent) {
              combinedContent += `<think>${reasoningContent}</think>\n\n`;
            }
            combinedContent += accumulatedContent;

            // Update the assistant message
            assistantMessage = {
              ...assistantMessage,
              content: [{ type: "text", text: combinedContent }],
            };

            setState((prevState) => ({
              ...prevState,
              messages: [...prevState.messages.slice(0, -1), assistantMessage],
              commandData: uiEventData, // Store the UI event data for parent components
            }));
            continue;
          } else if (event.type === "command") {
            // Handle UI command from side channel
            const commandData = event.data as any;

            // Update assistant message to indicate command execution
            let commandText =
              "✅ **执行命令**: " + (commandData.type || "未知命令");
            if (
              commandData.params &&
              Object.keys(commandData.params).length > 0
            ) {
              commandText += `\n\`\`\`json\n${JSON.stringify(commandData.params, null, 2)}\n\`\`\``;
            }

            // Add command visualization to the accumulated content
            accumulatedContent +=
              (accumulatedContent ? "\n\n" : "") + commandText;

            // Create combined content with reasoning using think tags
            let combinedContent = "";
            if (showReasoning && reasoningContent) {
              combinedContent += `</think>${reasoningContent}</think>\n\n`;
            }
            combinedContent += accumulatedContent;

            // Update the assistant message
            assistantMessage = {
              ...assistantMessage,
              content: [{ type: "text", text: combinedContent }],
            };

            setState((prevState) => ({
              ...prevState,
              messages: [...prevState.messages.slice(0, -1), assistantMessage],
              commandData: commandData, // Store the command data
            }));
            continue;
          } else if (event.type === "error") {
            const errorMsg =
              typeof event.data === "object" && "error" in event.data
                ? (event.data as { error: string }).error
                : "Unknown error";
            setState((prevState) => ({
              ...prevState,
              isRunning: false,
              error: errorMsg,
            }));
            return;
          } else if (event.type === "done") {
            // Streaming complete
            setState((prevState) => ({
              ...prevState,
              isRunning: false,
            }));
            return;
          }
        }
      } catch (error) {
        if (error instanceof Error && error.name === "AbortError") {
          // Request was aborted, this is expected
          setState((prevState) => ({
            ...prevState,
            isRunning: false,
          }));
          return;
        }

        const errorMessage =
          error instanceof Error ? error.message : "Unknown error";
        setState((prevState) => ({
          ...prevState,
          isRunning: false,
          error: errorMessage,
        }));
      }
    },
    [],
  );

  const stop = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
  }, []);

  const clear = useCallback(() => {
    if (state.isRunning) {
      stop();
    }
    setState({
      messages: [],
      isRunning: false,
      error: null,
    });
  }, [state.isRunning, stop]);

  const runtime: Runtime = {
    ...state,
    append,
    stop,
    clear,
  };

  return (
    <RuntimeContext.Provider value={runtime}>
      {children}
    </RuntimeContext.Provider>
  );
}

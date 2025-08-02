import { useState, useEffect, useRef, useMemo } from "react";
import { useLLM } from "@/hooks/util-hooks/useLLM.tsx";
import { Markdown } from "../LumenMarkdown/Markdown";
import { LumenAvatar } from "../LumenAvatar/LumenAvatar";

interface LumenWikiProps {
  request: string;
  modelId?: string;
}

export function LumenWiki({
  request,
  modelId = "Qwen3-4B-q4f16_1-MLC",
}: LumenWikiProps) {
  const {
    isInitializing,
    isGenerating,
    conversation,
    generateAnswer,
    setSystemPrompt,
  } = useLLM();

  const [open, setOpen] = useState(false);
  const [responded, setResponded] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);

  // 初始化 system prompt
  useEffect(() => {
    setSystemPrompt(
      "You are a helpful AI assistant that provides informative and concise responses about various topics. Be friendly and engaging in your responses.",
    );
  }, [setSystemPrompt]);

  // 取出 conversation 中最后一条 assistant 消息
  const lastAssistant = useMemo(() => {
    // 倒序查找第一个 role === "assistant"
    for (let i = conversation.length - 1; i >= 0; i--) {
      if (conversation[i].role === "assistant") {
        return conversation[i].content;
      }
    }
    return "";
  }, [conversation]);

  // 点击按钮：切面板 & 只触发一次生成
  const handleToggle = async () => {
    if (open) {
      setOpen(false);
      return;
    }
    setOpen(true);

    if (responded) return;

    await generateAnswer(request + "/no_think", {
      modelId,
      temperature: 0.7,
    });
    setResponded(true);
  };

  // 回答更新时自动滚动到底部
  useEffect(() => {
    const el = contentRef.current;
    if (el) {
      el.scrollTop = el.scrollHeight;
    }
  }, [lastAssistant]);

  return (
    <div className="w-full max-w-4xl mx-auto p-4">
      <div className="flex items-center mb-6">
        <div className="inline-block text-left">
          <button
            type="button"
            onClick={handleToggle}
            disabled={isInitializing}
            className={`
              px-3 py-1 rounded border transition
              ${
                isInitializing
                  ? "bg-gray-200 text-gray-500 border-gray-300 cursor-not-allowed"
                  : "bg-white text-blue-600 border-blue-600 hover:bg-blue-50"
              }
            `}
          >
            ℹ️
          </button>

          {open && (
            <div className="mt-2 w-80 bg-white border border-gray-200 rounded shadow-lg p-4 z-20 flex flex-col">
              <div className="flex justify-items-center items-center">
                <LumenAvatar start={isGenerating} size={0.2} />
                {isGenerating && (
                  <span className="ml-2 animate-pulse text-xs text-gray-500">
                    Generating...
                  </span>
                )}
              </div>

              <div
                ref={contentRef}
                className="flex-1 overflow-y-auto max-h-64 scroll-smooth text-sm text-gray-600"
              >
                {lastAssistant ? (
                  <Markdown content={lastAssistant} />
                ) : (
                  <div className="text-center text-gray-400">Loading…</div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

import { Brain, ChevronDown } from "lucide-react";
import { useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import type { ReasoningBlock as ReasoningBlockData } from "../../../model/chatTypes";

interface ReasoningBlockProps {
  block: ReasoningBlockData;
}

/** Collapsible reasoning trace. While streaming (no duration yet) it shows a
 * spinner and stays expanded; once closed it collapses to a one-line summary. */
export function ReasoningBlock({ block }: ReasoningBlockProps) {
  const { t } = useI18n();
  const isThinking = block.durationS === undefined;
  const [isOpen, setIsOpen] = useState<boolean | null>(null);
  const expanded = isOpen ?? isThinking;

  const label = isThinking
    ? t("lumilio.markdown.think.thinking")
    : t("lumilio.markdown.think.thoughtFor", { time: block.durationS });

  return (
    <div className="my-4">
      <button
        onClick={() => setIsOpen(!expanded)}
        className="flex items-center gap-2 text-sm text-base-content/50 hover:text-base-content/70 transition-colors duration-200 cursor-pointer"
      >
        <span className="relative flex items-center justify-center w-5 h-5">
          {isThinking ? (
            <span className="w-3.5 h-3.5 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
          ) : (
            <Brain className="w-4 h-4" strokeWidth={1.5} />
          )}
        </span>
        <span className="font-medium">{label}</span>
        <ChevronDown
          className={`w-4 h-4 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
          strokeWidth={1.5}
        />
      </button>

      {expanded && (
        <div className="mt-3 pl-7 border-l-2 border-base-300 text-base-content/70 text-sm leading-relaxed whitespace-pre-wrap">
          {block.text}
        </div>
      )}
    </div>
  );
}

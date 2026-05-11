import { Brain, ChevronDown } from "lucide-react";
import React, { useEffect, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";

interface ThinkBlockProps {
  open?: boolean;
  className?: string;
  children?: React.ReactNode;
  [key: string]: any;
}

export const ThinkBlock: React.FC<ThinkBlockProps> = ({
  open = false,
  className = "",
  children,
  ...props
}) => {
  const { t } = useI18n();
  const [isOpen, setIsOpen] = useState(open);
  const [thinkingTime, setThinkingTime] = useState<number | null>(null);
  const startTimeRef = useRef<number>(Date.now());
  const prevOpenRef = useRef(open);

  // When the block transitions from open (streaming) to closed (complete),
  // calculate the elapsed thinking time.
  useEffect(() => {
    // Record start time when block first opens
    if (open && !prevOpenRef.current) {
      startTimeRef.current = Date.now();
      setThinkingTime(null);
    }

    // When block closes, calculate duration
    if (!open && prevOpenRef.current) {
      const elapsed = Math.round((Date.now() - startTimeRef.current) / 1000);
      setThinkingTime(elapsed > 0 ? elapsed : 1);
      setIsOpen(false);
    }

    prevOpenRef.current = open;
  }, [open]);

  // Sync with external open state if it changes
  useEffect(() => {
    setIsOpen(open);
  }, [open]);

  const toggleOpen = () => {
    setIsOpen(!isOpen);
  };

  // Filter out summary elements from children
  const childrenArray = React.Children.toArray(children);
  const contentElements = childrenArray.filter(
    (child) => !React.isValidElement(child) || child.type !== "summary",
  );

  // Determine the label to show
  const isCurrentlyThinking = open;
  const label = isCurrentlyThinking
    ? t("lumilio.markdown.think.thinking") // "Thinking..."
    : thinkingTime !== null
      ? t("lumilio.markdown.think.thoughtFor", { time: thinkingTime }) // "Thought for {time}s"
      : t("lumilio.markdown.think.defaultSummary");

  return (
    <div className={`my-5 ${className}`} {...props}>
      {/* Flat thinking indicator bar */}
      <button
        onClick={toggleOpen}
        className="flex items-center gap-2 text-sm text-base-content/50 hover:text-base-content/70 transition-colors duration-200 cursor-pointer group"
      >
        <span className="relative flex items-center justify-center w-5 h-5">
          {isCurrentlyThinking ? (
            <span className="absolute inset-0 flex items-center justify-center">
              <span className="w-3.5 h-3.5 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
            </span>
          ) : (
            <Brain className="w-4 h-4" strokeWidth={1.5} />
          )}
        </span>
        <span className="font-medium">{label}</span>
        <ChevronDown
          className={`w-4 h-4 transition-transform duration-200 ${
            isOpen ? "rotate-180" : ""
          }`}
          strokeWidth={1.5}
        />
      </button>

      {/* Collapsible reasoning content */}
      {isOpen && (
        <div className="mt-3 pl-7 border-l-2 border-base-300 text-base-content/70 text-sm leading-relaxed">
          {contentElements}
        </div>
      )}
    </div>
  );
};

import React from "react";
import { TriggerPhase, MentionType } from "./types";
import { IconCommand } from "./data/mockData";

interface MentionMenuProps {
  phase: TriggerPhase;
  activeMentionType: MentionType | null;
  menuPos: { top: number; left: number } | null;
  options: any[];
  selectedIndex: number;
  setSelectedIndex: (index: number) => void;
  onSelectOption: (option: any) => void;
  editorRef: React.RefObject<HTMLDivElement | null>;
}

export function MentionMenu({
  phase,
  activeMentionType,
  menuPos,
  options,
  selectedIndex,
  setSelectedIndex,
  onSelectOption,
  editorRef,
}: MentionMenuProps) {
  if (phase === "IDLE" || !menuPos || options.length === 0) return null;

  // 计算相对于编辑器的位置
  const getRelativePosition = () => {
    if (!editorRef.current) return { top: 0, left: 0 };

    const editorRect = editorRef.current.getBoundingClientRect();
    return {
      top: menuPos.top - editorRect.top - 10, // 调整为相对于编辑器的位置
      left: menuPos.left - editorRect.left + 16,
    };
  };

  const position = getRelativePosition();
  const menuTitle =
    phase === "SELECT_TYPE"
      ? "Select Type"
      : phase === "COMMAND"
        ? "Commands"
        : `Select ${activeMentionType}`;

  return (
    <div
      className="absolute z-50 w-64 bg-base-100 rounded-lg shadow-xl border border-base-300 overflow-hidden"
      style={{
        top: position.top,
        left: position.left,
        transform: "translateY(-100%)",
      }}
    >
      <div className="bg-base-200 px-3 py-2 text-xs font-semibold text-base-content/70 uppercase tracking-wider border-b border-base-300 flex justify-between">
        <span>{menuTitle}</span>
        <span className="text-base-content/50">Tab ↹</span>
      </div>
      <div className="overflow-y-auto max-h-48 py-1">
        {options.map((opt, idx) => (
          <div
            key={opt.id || opt.type}
            className={`flex items-center px-4 py-2 cursor-pointer text-sm ${
              idx === selectedIndex
                ? "bg-primary/10 text-primary"
                : "text-base-content hover:bg-base-200"
            }`}
            onMouseEnter={() => setSelectedIndex(idx)}
            onClick={() => onSelectOption(opt)}
          >
            <span
              className={`mr-2 ${idx === selectedIndex ? "text-primary" : "text-base-content/50"}`}
            >
              {opt.icon || (phase === "COMMAND" ? <IconCommand /> : null)}
            </span>
            <span className="flex-1">{opt.label}</span>
            {opt.meta && (
              <span className="text-xs text-base-content/50 ml-2">
                {opt.meta}
              </span>
            )}
            {phase === "SELECT_TYPE" && (
              <span className="text-base-content/50 text-xs ml-1">›</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

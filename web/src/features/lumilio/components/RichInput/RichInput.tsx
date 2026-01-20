// Lumilio-Photos/web/src/features/lumilio/rich-input/RichInput.tsx
import React, { useRef, useCallback, useEffect } from "react";
import { useRichInput } from "./RichInputProvider";
import { MentionEntity, MentionTypeOption, MentionType } from "./types";
import {
  parseContentToPayload,
  calculateMenuPosition,
  insertPill,
  clearEditor,
} from "./utils";

export interface RichInputProps {
  /** 输入框占位符文本 */
  placeholder?: string;
  /** 可用的提及类型（如 album, tag, camera 等） */
  mentionTypes?: MentionTypeOption[];
  /** 根据提及类型获取实体列表的函数 */
  getEntitiesByType?: (type: MentionType) => MentionEntity[];
  /** 可用的命令列表 */
  commands?: MentionEntity[];
  /** 提交回调 */
  onSubmit?: (payload: string) => void;
  /** 是否禁用提交 */
  isSubmitting?: boolean;
  /** 自定义样式类名 */
  className?: string;
}

export const RichInput: React.FC<RichInputProps> = ({
  placeholder = "Ask Lumilio... (Type @ or /)",
  mentionTypes = [],
  getEntitiesByType,
  commands = [],
  onSubmit,
  isSubmitting = false,
  className = "",
}) => {
  const editorRef = useRef<HTMLDivElement>(null);
  const { state, dispatch } = useRichInput();

  const updatePayloadPreview = useCallback(() => {
    if (editorRef.current) {
      dispatch({
        type: "SET_PAYLOAD",
        payload: parseContentToPayload(editorRef.current),
      });
    }
  }, [dispatch]);

  const handleSubmitClick = useCallback(() => {
    if (state.payload.trim() && !isSubmitting && onSubmit) {
      onSubmit(state.payload);
      clearEditor(editorRef);
      dispatch({ type: "SET_PAYLOAD", payload: "" });
    }
  }, [state.payload, isSubmitting, onSubmit, dispatch]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const { phase, options, selectedIndex } = state;

      if (phase !== "IDLE" && options.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          dispatch({
            type: "SET_SELECTED_INDEX",
            payload: (selectedIndex + 1) % options.length,
          });
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          dispatch({
            type: "SET_SELECTED_INDEX",
            payload: (selectedIndex - 1 + options.length) % options.length,
          });
        } else if (e.key === "Enter" || e.key === "Tab") {
          e.preventDefault();
          const selected = options[selectedIndex];
          if (phase === "SELECT_TYPE") {
            dispatch({
              type: "SET_ACTIVE_MENTION_TYPE",
              payload: selected.type as MentionType,
            });
            dispatch({ type: "SET_PHASE", payload: "SELECT_ENTITY" });
            dispatch({ type: "SET_SELECTED_INDEX", payload: 0 });
          } else {
            insertPill(selected, () => {
              dispatch({ type: "RESET_EDITOR" });
              updatePayloadPreview();
            });
          }
        } else if (e.key === "Escape") {
          e.preventDefault();
          dispatch({ type: "RESET_EDITOR" });
        }
      } else {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          handleSubmitClick();
        }
      }
    },
    [state, dispatch, updatePayloadPreview, handleSubmitClick],
  );

  const handleInput = useCallback(() => {
    updatePayloadPreview();

    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0) return;

    const range = selection.getRangeAt(0);
    const textNode = range.startContainer;

    if (textNode.nodeType !== Node.TEXT_NODE) return;

    const content = textNode.textContent || "";
    const offset = range.startOffset;
    const lastChar = content.slice(0, offset).slice(-1);

    if (lastChar === "@" || lastChar === "/") {
      const menuPos = calculateMenuPosition(range, editorRef);
      if (menuPos) {
        dispatch({ type: "SET_MENU_POSITION", payload: menuPos });
        dispatch({
          type: "SET_PHASE",
          payload: lastChar === "/" ? "COMMAND" : "SELECT_TYPE",
        });
      }
    } else if (state.phase !== "IDLE" && content.includes(" ")) {
      dispatch({ type: "RESET_EDITOR" });
    }
  }, [updatePayloadPreview, state.phase, dispatch]);

  const handleOptionClick = useCallback(
    (option: MentionEntity) => {
      if (state.phase === "SELECT_TYPE") {
        dispatch({
          type: "SET_ACTIVE_MENTION_TYPE",
          payload: option.type as MentionType,
        });
        dispatch({
          type: "SET_PHASE",
          payload: "SELECT_ENTITY",
        });
        dispatch({
          type: "SET_SELECTED_INDEX",
          payload: 0,
        });
      } else {
        insertPill(option, () => {
          dispatch({
            type: "RESET_EDITOR",
          });
          updatePayloadPreview();
        });
      }
    },
    [state.phase, dispatch, updatePayloadPreview],
  );

  const handleOptionMouseEnter = useCallback(
    (idx: number) => {
      dispatch({
        type: "SET_SELECTED_INDEX",
        payload: idx,
      });
    },
    [dispatch],
  );

  useEffect(() => {
    let newOptions: MentionEntity[] = [];
    if (state.phase === "SELECT_TYPE") {
      newOptions = mentionTypes.map((opt) => ({
        ...opt,
        id: opt.type,
        type: opt.type,
      }));
    } else if (
      state.phase === "SELECT_ENTITY" &&
      state.activeMentionType &&
      getEntitiesByType
    ) {
      newOptions = getEntitiesByType(state.activeMentionType);
    } else if (state.phase === "COMMAND") {
      newOptions = commands;
    }
    dispatch({ type: "SET_OPTIONS", payload: newOptions });
  }, [
    state.phase,
    state.activeMentionType,
    mentionTypes,
    commands,
    getEntitiesByType,
    dispatch,
  ]);

  return (
    <div className={`relative ${className}`}>
      {/* Input container */}
      <div className="relative bg-base-200 rounded-xl border border-base-300 focus-within:ring-2 focus-within:ring-primary focus-within:bg-base-100 transition-all">
        {/* ContentEditable Input */}
        <div
          ref={editorRef}
          contentEditable
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          className="w-full max-h-40 overflow-y-auto p-4 pr-14 focus:outline-none text-base leading-relaxed empty:before:content-[attr(data-placeholder)] empty:before:text-base-content/50"
          data-placeholder={placeholder}
          suppressContentEditableWarning
        />

        {/* Integrated Send Button */}
        <div className="absolute right-3 bottom-3 flex items-center">
          <button
            onClick={handleSubmitClick}
            disabled={isSubmitting || !state.payload.trim()}
            className="btn btn-primary btn-square btn-sm"
          >
            {isSubmitting ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="w-4 h-4"
              >
                <path d="M3.478 2.405a.75.75 0 00-.926.94l2.432 7.905H13.5a.75.75 0 010 1.5H4.984l-2.432 7.905a.75.75 0 00.926.94 60.519 60.519 0 0018.445-8.986.75.75 0 000-1.218A60.517 60.517 0 003.478 2.405z" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Floating Menu */}
      {state.phase !== "IDLE" && state.menuPos && (
        <div
          className="absolute z-50 w-72 bg-base-100 rounded-lg shadow-xl border border-base-300 overflow-hidden"
          style={{
            bottom: "100%",
            marginBottom: "8px",
            left: 0,
          }}
        >
          <div className="bg-base-200 px-3 py-2 text-xs font-semibold text-base-content/70 uppercase tracking-wider border-b border-base-300 flex justify-between">
            <span>{state.phase === "COMMAND" ? "Commands" : "Mention"}</span>
            <span className="font-mono">Tab ↹</span>
          </div>
          <div className="overflow-y-auto max-h-48 py-1">
            {state.options.map((option, idx) => (
              <div
                key={option.id}
                className={`flex items-center px-3 py-2 cursor-pointer text-sm transition-colors ${
                  idx === state.selectedIndex
                    ? "bg-primary text-primary-content"
                    : "text-base-content hover:bg-base-200"
                }`}
                onMouseEnter={() => handleOptionMouseEnter(idx)}
                onClick={() => handleOptionClick(option)}
              >
                <span className="mr-3 opacity-50">
                  {option.type === "command" ? "/" : "@"}
                </span>
                <span className="flex-1 font-medium">{option.label}</span>
                {option.desc && (
                  <span className="text-xs opacity-70 ml-2 truncate">
                    {option.desc}
                  </span>
                )}
              </div>
            ))}
            {state.options.length === 0 && (
              <div className="px-4 py-3 text-sm text-base-content/50 text-center">
                No options
              </div>
            )}
          </div>
        </div>
      )}

      {/* Debug Payload Preview */}
      {import.meta.env.DEV && (
        <div className="mt-2 text-[10px] text-base-content/30 font-mono truncate px-1">
          Payload: {state.payload || "(empty)"}
        </div>
      )}
    </div>
  );
};

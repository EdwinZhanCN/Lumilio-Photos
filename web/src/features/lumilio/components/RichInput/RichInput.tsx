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

/**
 * RichInput 组件
 *
 * 一个支持提及（@）和命令（/）的富文本输入框
 *
 * @example
 * ```tsx
 * <RichInput
 *   placeholder="Type @ to mention albums..."
 *   mentionTypes={MENTION_TYPES}
 *   getEntitiesByType={(type) => MOCK_ENTITIES[type] || []}
 *   commands={COMMANDS}
 *   onSubmit={(payload) => console.log(payload)}
 * />
 * ```
 */
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

  // 更新 payload 预览
  const updatePayloadPreview = useCallback(() => {
    if (editorRef.current) {
      dispatch({
        type: "SET_PAYLOAD",
        payload: parseContentToPayload(editorRef.current),
      });
    }
  }, [dispatch]);

  // 根据 phase 和 activeMentionType 加载选项
  useEffect(() => {
    let newOptions: MentionEntity[] = [];

    if (state.phase === "SELECT_TYPE") {
      newOptions = mentionTypes.map((opt, idx) => ({
        ...opt,
        id: `type-${opt.type}-${idx}`,
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

    dispatch({
      type: "SET_OPTIONS",
      payload: newOptions,
    });
  }, [
    state.phase,
    state.activeMentionType,
    mentionTypes,
    commands,
    getEntitiesByType,
    dispatch,
  ]);

  // 处理输入事件
  const handleInput = useCallback(() => {
    updatePayloadPreview();

    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0) return;

    const range = selection.getRangeAt(0);
    const textNode = range.startContainer;

    if (textNode.nodeType !== Node.TEXT_NODE) return;

    const content = textNode.textContent || "";
    const offset = range.startOffset;
    const textBeforeCursor = content.slice(0, offset);
    const lastChar = textBeforeCursor.slice(-1);

    if (lastChar === "@") {
      const menuPos = calculateMenuPosition(range, editorRef);
      if (menuPos) {
        dispatch({
          type: "SET_MENU_POSITION",
          payload: menuPos,
        });
        dispatch({
          type: "SET_PHASE",
          payload: "SELECT_TYPE",
        });
      }
    } else if (lastChar === "/") {
      const menuPos = calculateMenuPosition(range, editorRef);
      if (menuPos) {
        dispatch({
          type: "SET_MENU_POSITION",
          payload: menuPos,
        });
        dispatch({
          type: "SET_PHASE",
          payload: "COMMAND",
        });
      }
    } else if (state.phase !== "IDLE") {
      if (lastChar === " " && state.phase === "SELECT_TYPE") {
        // 如果在选择类型阶段输入空格，则取消
        dispatch({
          type: "SET_PHASE",
          payload: "IDLE",
        });
        dispatch({
          type: "SET_MENU_POSITION",
          payload: null,
        });
      }
    }
  }, [updatePayloadPreview, state.phase, dispatch]);

  // 处理键盘事件
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const { phase, options, selectedIndex } = state;

      // 如果菜单打开且有选项
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
            // 选择类型后，进入实体选择阶段
            dispatch({
              type: "SET_ACTIVE_MENTION_TYPE",
              payload: selected.type as MentionType,
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
            // 插入实体
            insertPill(selected, () => {
              // 插入后重置状态
              dispatch({
                type: "RESET_EDITOR",
              });
              updatePayloadPreview();
            });
          }
        } else if (e.key === "Escape") {
          e.preventDefault();
          dispatch({
            type: "SET_PHASE",
            payload: "IDLE",
          });
          dispatch({
            type: "SET_MENU_POSITION",
            payload: null,
          });
        }
      } else {
        // 正常输入模式
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          if (state.payload.trim() && !isSubmitting) {
            onSubmit?.(state.payload);
            clearEditor(editorRef);
            dispatch({
              type: "SET_PAYLOAD",
              payload: "",
            });
          }
        }
      }
    },
    [state, dispatch, updatePayloadPreview, onSubmit, isSubmitting],
  );

  // 处理选项点击
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

  // 处理提交按钮点击
  const handleSubmitClick = useCallback(() => {
    if (state.payload.trim() && !isSubmitting) {
      onSubmit?.(state.payload);
      clearEditor(editorRef);
      dispatch({
        type: "SET_PAYLOAD",
        payload: "",
      });
    }
  }, [state.payload, isSubmitting, onSubmit, dispatch]);

  // 处理鼠标悬停选项
  const handleOptionMouseEnter = useCallback(
    (idx: number) => {
      dispatch({
        type: "SET_SELECTED_INDEX",
        payload: idx,
      });
    },
    [dispatch],
  );

  return (
    <div className={`relative ${className}`}>
      {/* 输入框容器 */}
      <div className="relative bg-slate-100 rounded-xl border border-slate-200 focus-within:ring-2 focus-within:ring-blue-500 focus-within:bg-white transition-all">
        {/* ContentEditable 输入框 */}
        <div
          ref={editorRef}
          contentEditable
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          className="w-full max-h-32 overflow-y-auto p-3 focus:outline-none text-sm leading-relaxed empty:before:content-[attr(data-placeholder)] empty:before:text-slate-400"
          data-placeholder={placeholder}
          suppressContentEditableWarning
        />

        {/* 底部工具栏 */}
        <div className="flex justify-between items-center px-2 pb-2 mt-1">
          <div className="flex gap-2 text-xs text-slate-400">
            <span className="flex items-center gap-1 bg-white border rounded px-1.5 py-0.5">
              @ Mention
            </span>
            <span className="flex items-center gap-1 bg-white border rounded px-1.5 py-0.5">
              / Command
            </span>
          </div>
          <button
            onClick={handleSubmitClick}
            disabled={isSubmitting || !state.payload.trim()}
            className={`p-2 rounded-lg transition-colors ${
              isSubmitting || !state.payload.trim()
                ? "bg-slate-200 text-slate-400 cursor-not-allowed"
                : "bg-blue-600 text-white hover:bg-blue-700 shadow-sm"
            }`}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <line x1="22" y1="2" x2="11" y2="13" />
              <polygon points="22 2 15 22 11 13 2 9 22 2" />
            </svg>
          </button>
        </div>
      </div>

      {/* 悬浮菜单 */}
      {state.phase !== "IDLE" && state.menuPos && (
        <div
          className="absolute z-50 w-64 bg-white rounded-lg shadow-xl border border-slate-200 overflow-hidden"
          style={{
            top: state.menuPos.top,
            left: state.menuPos.left,
          }}
        >
          {/* 菜单标题 */}
          <div className="bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-500 uppercase tracking-wider border-b flex justify-between">
            <span>
              {state.phase === "SELECT_TYPE"
                ? "Select Type"
                : state.phase === "COMMAND"
                  ? "Commands"
                  : `Select ${state.activeMentionType}`}
            </span>
            <span className="text-slate-400">Tab ↹</span>
          </div>

          {/* 选项列表 */}
          <div className="overflow-y-auto max-h-48 py-1">
            {state.options.map((option: MentionEntity, idx: number) => (
              <div
                key={option.id || option.type}
                className={`flex items-center px-4 py-2 cursor-pointer text-sm ${
                  idx === state.selectedIndex
                    ? "bg-blue-50 text-blue-700"
                    : "text-slate-700 hover:bg-slate-50"
                }`}
                onMouseEnter={() => handleOptionMouseEnter(idx)}
                onClick={() => handleOptionClick(option)}
              >
                {/* 图标 */}
                <span
                  className={`mr-2 ${
                    idx === state.selectedIndex
                      ? "text-blue-500"
                      : "text-slate-400"
                  }`}
                >
                  {option.icon}
                </span>

                {/* 标签 */}
                <span className="flex-1">{option.label}</span>

                {/* 描述 */}
                {option.desc && (
                  <span className="text-xs text-slate-400 ml-2">
                    {option.desc}
                  </span>
                )}

                {/* 子菜单指示器 */}
                {state.phase === "SELECT_TYPE" && (
                  <span className="text-slate-300 text-xs">›</span>
                )}
              </div>
            ))}

            {/* 空状态 */}
            {state.options.length === 0 && (
              <div className="px-4 py-3 text-sm text-slate-400 text-center">
                No options available
              </div>
            )}
          </div>
        </div>
      )}

      {/* Payload 预览（调试用） */}
      <div className="mt-2 text-[10px] text-slate-300 font-mono truncate px-1">
        Payload: {state.payload || "(empty)"}
      </div>
    </div>
  );
};

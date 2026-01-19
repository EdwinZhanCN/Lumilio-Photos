import { useState, useEffect, useCallback, useRef } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  Image as ImageIcon,
  Hash,
  Camera,
  Aperture,
  MapPin,
  Command,
} from "lucide-react";
import {
  MentionEntity,
  MentionType,
  TriggerPhase,
  MenuPosition,
} from "../types";
import { useAgentTools } from "../data/mockData";
// Temporarily disabled imports
// import { MENTION_TYPES, MOCK_ENTITIES } from "../data/mockData";
import { parseContentToPayload } from "../utils";

export function useRichMention() {
  const editorRef = useRef<HTMLDivElement>(null);
  const [phase, setPhase] = useState<TriggerPhase>("IDLE");
  const [activeMentionType, setActiveMentionType] =
    useState<MentionType | null>(null);
  const [menuPos, setMenuPos] = useState<MenuPosition | null>(null);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [options, setOptions] = useState<any[]>([]);
  const [payload, setPayload] = useState("");

  // Get agent tools for slash commands
  const agentTools = useAgentTools();

  // 选项加载逻辑
  useEffect(() => {
    if (phase === "SELECT_TYPE") {
      // Temporarily disabled @ mention functionality
      // setOptions(MENTION_TYPES);
      setOptions([]);
    } else if (phase === "SELECT_ENTITY" && activeMentionType) {
      // Temporarily disabled @ mention functionality
      // setOptions(MOCK_ENTITIES[activeMentionType] || []);
      setOptions([]);
    } else if (phase === "COMMAND") {
      setOptions(agentTools);
    } else {
      setOptions([]);
    }
    setSelectedIndex(0);
  }, [phase, activeMentionType, agentTools]);

  const updatePayloadPreview = useCallback(() => {
    if (editorRef.current) {
      setPayload(parseContentToPayload(editorRef.current));
    }
  }, []);

  // 插入胶囊 (Pill)
  const insertPill = useCallback(
    (entity: MentionEntity) => {
      const selection = window.getSelection();
      if (!selection || selection.rangeCount === 0 || !editorRef.current)
        return;

      const range = selection.getRangeAt(0);
      const textNode = range.startContainer;

      // 1. 删除触发符 (@ 或 /)
      if (textNode.nodeType === Node.TEXT_NODE && textNode.textContent) {
        const content = textNode.textContent;
        const lastAtIndex = content.lastIndexOf("@");
        const lastSlashIndex = content.lastIndexOf("/");
        const cutIndex = Math.max(lastAtIndex, lastSlashIndex);

        if (cutIndex !== -1) {
          textNode.textContent = content.substring(0, cutIndex);
          range.setStart(textNode, cutIndex);
          range.setEnd(textNode, cutIndex);
        }
      }

      // 2. 创建 Pill DOM
      const span = document.createElement("span");
      span.contentEditable = "false";
      span.className =
        "inline-flex items-center px-2 py-0.5 mx-1 rounded-full bg-primary/20 text-primary text-sm font-medium select-none cursor-default border border-primary/30 align-middle";
      span.setAttribute("data-mention-id", entity.id);
      span.setAttribute("data-mention-type", entity.type);
      span.setAttribute("data-mention-label", entity.label);

      // 3. 使用 Lucide Icon 生成 SVG 字符串
      let IconComponent = null;
      switch (entity.type) {
        case "album":
          IconComponent = ImageIcon;
          break;
        case "tag":
          IconComponent = Hash;
          break;
        case "camera":
          IconComponent = Camera;
          break;
        case "lens":
          IconComponent = Aperture;
          break;
        case "location":
          IconComponent = MapPin;
          break;
        case "command":
          IconComponent = Command;
          break;
        default:
          break;
      }

      const iconSvg = IconComponent
        ? renderToStaticMarkup(
            <IconComponent
              size={12}
              strokeWidth={2}
              style={{
                marginRight: 4,
                display: "inline-block",
                verticalAlign: "text-bottom",
              }}
            />,
          )
        : "";

      span.innerHTML = `${iconSvg}${entity.label}`;

      // 4. 插入 Pill
      range.insertNode(span);

      // 5. 将光标移动到 Pill 之后
      range.setStartAfter(span);
      range.setEndAfter(span);

      // 6. 插入空格
      const space = document.createTextNode("\u00A0");
      range.insertNode(space);

      // 7. 再次移动光标到空格之后
      range.setStartAfter(space);
      range.setEndAfter(space);

      // 8. 更新 Selection
      selection.removeAllRanges();
      selection.addRange(range);

      // 重置状态
      setPhase("IDLE");
      setMenuPos(null);
      setActiveMentionType(null);
      updatePayloadPreview();

      editorRef.current.focus();
    },
    [updatePayloadPreview],
  );

  // 处理输入事件
  const handleInput = useCallback(() => {
    updatePayloadPreview();

    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0 || !editorRef.current) return;

    const range = selection.getRangeAt(0);
    const textNode = range.startContainer;

    if (textNode.nodeType !== Node.TEXT_NODE) return;

    const content = textNode.textContent || "";
    const offset = range.startOffset;
    const textBeforeCursor = content.slice(0, offset);
    const lastChar = textBeforeCursor.slice(-1);

    const editorRect = editorRef.current.getBoundingClientRect();

    // Temporarily disabled @ mention functionality
    /* if (lastChar === "@") {
      const rects = range.getClientRects();
      const rect = rects.length > 0 ? rects[0] : range.getBoundingClientRect();
      const top = rect.top || editorRect.top + 20;
      const left = rect.left || editorRect.left + 20;

      setMenuPos({
        top: top + 20,
        left: left,
      });
      setPhase("SELECT_TYPE");
    } else
    */
    if (lastChar === "/") {
      const rects = range.getClientRects();
      const rect = rects.length > 0 ? rects[0] : range.getBoundingClientRect();
      const top = rect.top || editorRect.top + 20;
      const left = rect.left || editorRect.left + 20;

      setMenuPos({
        top: top + 20,
        left: left,
      });
      setPhase("COMMAND");
    }
    // Temporarily disabled @ mention functionality
    /*
    else if (phase !== "IDLE") {
      if (lastChar === " " && phase === "SELECT_TYPE") {
        setPhase("IDLE");
        setMenuPos(null);
      }
    }
    */
  }, [phase, updatePayloadPreview]);

  // 处理键盘事件
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (phase !== "IDLE" && options.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          setSelectedIndex((prev) => (prev + 1) % options.length);
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setSelectedIndex(
            (prev) => (prev - 1 + options.length) % options.length,
          );
        } else if (e.key === "Enter" || e.key === "Tab") {
          e.preventDefault();
          const selected = options[selectedIndex];

          // Temporarily disabled @ mention functionality
          // if (phase === "SELECT_TYPE") {
          //   setActiveMentionType(selected.type as MentionType);
          //   setPhase("SELECT_ENTITY");
          // } else {
          insertPill(selected);
          // }
        } else if (e.key === "Escape") {
          e.preventDefault();
          setPhase("IDLE");
          setMenuPos(null);
        }
      }
    },
    [phase, options, selectedIndex, insertPill],
  );

  // 处理提交
  const handleSubmit = useCallback(async () => {
    if (!payload.trim()) return;

    // Extract tool name from payload if it's a command
    let commandName = "";
    if (payload.startsWith("/")) {
      const parts = payload.split(" ");
      commandName = parts[0].substring(1); // Remove the leading slash
    }

    // 重置编辑器
    if (editorRef.current) editorRef.current.innerHTML = "";
    setPayload("");

    // Return command info for processing by parent
    return {
      text: payload,
      command: commandName ? { name: commandName } : undefined,
    };
  }, [payload]);

  // 清空内容
  const clearContent = useCallback(() => {
    if (editorRef.current) editorRef.current.innerHTML = "";
    setPayload("");
  }, []);

  return {
    editorRef,
    phase,
    menuPos,
    selectedIndex,
    options,
    payload,
    handleInput,
    handleKeyDown,
    handleSubmit,
    clearContent,
    setSelectedIndex,
    insertPill,
    setActiveMentionType,
    setPhase,
    activeMentionType,
  };
}

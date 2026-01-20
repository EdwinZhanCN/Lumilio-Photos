// Lumilio-Photos/web/src/features/lumilio/rich-input/utils.ts
import ReactDOMServer from "react-dom/server";

/**
 * 解析 contentEditable 容器的内容为结构化的 payload 字符串
 *
 * 将文本节点和提及元素（带有 data-mention-* 属性的元素）组合成格式化字符串。
 * 提及元素会被转换为格式: @[Label](Type:ID)
 *
 * @param container - contentEditable 的容器元素
 * @returns 解析后的 payload 字符串
 *
 * @example
 * ```tsx
 * // HTML 内容：
 * // "Hello @[Summer Trip](album:123)!"
 * //
 * // 返回：
 * // "Hello @[Summer Trip](album:123)"
 * ```
 */
export const parseContentToPayload = (container: HTMLDivElement): string => {
  let text = "";

  container.childNodes.forEach((node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      // 文本节点直接添加内容
      text += node.textContent;
    } else if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as HTMLElement;

      // 检查是否是提及元素（带有 data-mention-id 属性）
      if (el.hasAttribute("data-mention-id")) {
        const id = el.getAttribute("data-mention-id");
        const type = el.getAttribute("data-mention-type");
        const label = el.getAttribute("data-mention-label");

        // 生成格式: @[Label](Type:ID)
        if (id && type && label) {
          text += ` @[${label}](${type}:${id}) `;
        }
      } else {
        // 其他元素使用其文本内容
        text += el.innerText;
      }
    }
  });

  // 清理空格：将不间断空格替换为普通空格，并压缩多个连续空格
  return text
    .replace(/\u00A0/g, " ")
    .replace(/\s+/g, " ")
    .trim();
};

/**
 * 创建一个胶囊（Pill）元素
 *
 * 提及元素使用 contentEditable="false"，不可编辑
 *
 * @param entity - 要插入的提及实体
 * @returns 创建的 HTMLSpanElement
 *
 * @example
 * ```tsx
 * const pill = createPillElement({
 *   id: "123",
 *   label: "Summer Trip",
 *   type: "album",
 *   icon: <IconAlbum />
 * });
 * ```
 */
/**
 * 获取根据类型生成的默认图标 SVG
 *
 * @param type - 提及类型
 * @returns 图标 HTML 字符串
 */
const getDefaultIconByType = (type: string): string => {
  const iconStyle =
    "width: 12px; height: 12px; margin-right: 4px; display: inline-block; vertical-align: middle;";
  if (type === "command") {
    // Return a slash icon SVG
    return ``;
  }
  // Return the default at-symbol icon SVG
  return `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="${iconStyle}"><circle cx="12" cy="12" r="4"/><path d="M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-4 8"/></svg>`;
};

/**
 * 创建一个胶囊（Pill）元素
 *
 * 提及元素使用 contentEditable="false"，不可编辑
 *
 * @param entity - 要插入的提及实体
 * @returns 创建的 HTMLSpanElement
 *
 * @example
 * ```tsx
 * const pill = createPillElement({
 *   id: "123",
 *   label: "Summer Trip",
 *   type: "album",
 *   icon: <IconAlbum />
 * });
 * ```
 */
export const createPillElement = (entity: {
  id: string;
  label: string;
  type: string;
  icon?: React.ReactNode;
}): HTMLSpanElement => {
  const span = document.createElement("span");
  span.contentEditable = "false";
  span.className =
    "inline-flex items-center px-2 py-0.5 mx-1 rounded-full bg-blue-100 text-blue-700 text-sm font-medium select-none cursor-default border border-blue-200 align-middle";

  // 设置数据属性，用于后续解析
  span.setAttribute("data-mention-id", entity.id);
  span.setAttribute("data-mention-type", entity.type);
  span.setAttribute("data-mention-label", entity.label);

  // 使用传入的 icon，如果没有则根据 type 生成默认图标
  const iconHtml = entity.icon
    ? ReactDOMServer.renderToString(entity.icon)
    : getDefaultIconByType(entity.type);

  const displayLabel =
    entity.type === "command" ? `/${entity.label}` : entity.label;
  span.innerHTML = `${iconHtml}${displayLabel}`;

  return span;
};

/**
 * 计算菜单的显示位置
 *
 * 根据光标位置和编辑器位置，计算出悬浮菜单应该显示的位置
 *
 * @param range - 当前选中的 Range
 * @param editorRef - 编辑器的 ref
 * @returns 菜单位置 { top, left }
 */
export const calculateMenuPosition = (
  range: Range,
  editorRef: React.RefObject<HTMLDivElement | null>,
): { top: number; left: number } | null => {
  const rects = range.getClientRects();
  const rect = rects.length > 0 ? rects[0] : range.getBoundingClientRect();

  const editorRect = editorRef.current?.getBoundingClientRect();

  if (!editorRect) return null;

  const top = rect.top || editorRect.top + 20;
  const left = rect.left || editorRect.left + 20;

  return { top: top + 20, left };
};

/**
 * 查找文本节点中最后一个触发符（@ 或 /）的位置
 *
 * @param textNode - 文本节点
 * @param offset - 光标偏移量
 * @returns 最后一个触发符的索引，如果没有则返回 -1
 */
export const findLastTriggerChar = (
  textNode: Text,
  offset: number,
): { index: number; char: "@" | "/" } | null => {
  const content = textNode.textContent || "";

  const lastAtIndex = content.lastIndexOf("@");
  const lastSlashIndex = content.lastIndexOf("/");

  const cutIndex = Math.max(lastAtIndex, lastSlashIndex);

  if (cutIndex !== -1 && cutIndex < offset) {
    return {
      index: cutIndex,
      char: lastAtIndex > lastSlashIndex ? "@" : "/",
    };
  }

  return null;
};

/**
 * 将胶囊元素插入到编辑器中
 *
 * 此函数会：
 * 1. 删除触发符（@ 或 /）
 * 2. 创建并插入胶囊元素
 * 3. 将光标移动到胶囊后面并插入空格
 *
 * @param entity - 要插入的提及实体
 * @param onInsert - 插入后的回调函数
 */
export const insertPill = (
  entity: {
    id: string;
    label: string;
    type: string;
    icon?: React.ReactNode;
  },
  onInsert?: () => void,
): void => {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) return;

  const range = selection.getRangeAt(0);
  const textNode = range.startContainer;

  // 1. 删除触发符 (@ 或 /)
  if (textNode.nodeType === Node.TEXT_NODE && textNode.textContent) {
    const triggerInfo = findLastTriggerChar(
      textNode as Text,
      range.startOffset,
    );

    if (triggerInfo) {
      textNode.textContent = textNode.textContent.substring(
        0,
        triggerInfo.index,
      );
      // 更新 Range 到截断后的位置
      range.setStart(textNode, triggerInfo.index);
      range.setEnd(textNode, triggerInfo.index);
    }
  }

  // 2. 创建并插入 Pill DOM
  const span = createPillElement(entity);
  range.insertNode(span);

  // 3. 将光标移动到 Pill 之后
  range.setStartAfter(span);
  range.setEndAfter(span);

  // 4. 插入空格
  const space = document.createTextNode("\u00A0");
  range.insertNode(space);

  // 5. 再次移动光标到空格之后
  range.setStartAfter(space);
  range.setEndAfter(space);

  // 6. 更新 Selection
  selection.removeAllRanges();
  selection.addRange(range);

  // 7. 触发回调
  onInsert?.();
};

/**
 * 清空编辑器内容
 *
 * @param editorRef - 编辑器的 ref
 */
export const clearEditor = (
  editorRef: React.RefObject<HTMLDivElement | null>,
): void => {
  if (editorRef.current) {
    editorRef.current.innerHTML = "";
  }
};

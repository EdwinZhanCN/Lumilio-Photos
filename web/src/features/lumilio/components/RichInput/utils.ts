// Lumilio-Photos/web/src/features/lumilio/rich-input/utils.ts
import ReactDOMServer from "react-dom/server";

/** Parses contentEditable container content into a structured payload string.

  Combines text nodes and mention elements (with data-mention-* attributes) into
  a formatted string. Mention elements are converted to format: @[Label](Type:ID).

  Args:
    container: The contentEditable container element.

  Returns:
    The parsed payload string.

  Example:
    HTML content: "Hello @[Summer Trip](album:123)!"
    Returns: "Hello @[Summer Trip](album:123)"
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

/** Gets the default icon SVG based on type.

  Args:
    type: The mention type.

  Returns:
    The icon HTML string.
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

/** Creates a pill (capsule) element for mentions.

  Mention elements use contentEditable="false", making them non-editable.

  Args:
    entity: The mention entity to create a pill for, containing id, label, type,
            and optional icon.

  Returns:
    The created HTMLSpanElement representing the pill.

  Example:
    ```tsx
    const pill = createPillElement({
      id: "123",
      label: "Summer Trip",
      type: "album",
      icon: <IconAlbum />
    });
    ```
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

/** Calculates the menu display position.

  Computes the position where the floating menu should be displayed based on
  the cursor position and editor position.

  Args:
    range: The currently selected Range.
    editorRef: Reference to the editor element.

  Returns:
    The menu position as { top, left } or null if calculation fails.
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

/** Finds the position of the last trigger character (@ or /) in a text node.

  Args:
    textNode: The text node to search.
    offset: The cursor offset position.

  Returns:
    An object containing the index and character of the last trigger,
    or null if no trigger character is found.
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

/** Inserts a pill element into the editor.

  This function performs the following steps:
  1. Deletes the trigger character (@ or /)
  2. Creates and inserts the pill element
  3. Moves the cursor after the pill and inserts a space

  Args:
    entity: The mention entity to insert.
    onInsert: Optional callback function to execute after insertion.
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

/** Clears the editor content.

  Args:
    editorRef: Reference to the editor element to clear.
*/
export const clearEditor = (
  editorRef: React.RefObject<HTMLDivElement | null>,
): void => {
  if (editorRef.current) {
    editorRef.current.innerHTML = "";
  }
};

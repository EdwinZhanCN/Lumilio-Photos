// Lumilio-Photos/web/src/features/lumilio/rich-input/utils.ts

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

  // 生成图标 SVG（根据类型）
  let iconSvg = "";
  if (entity.type === "album")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>';
  else if (entity.type === "tag")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/></svg>';
  else if (entity.type === "camera")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>';
  else if (entity.type === "lens")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>';
  else if (entity.type === "location")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/><circle cx="12" cy="10" r="3"/></svg>';
  else if (entity.type === "command")
    iconSvg =
      '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px"><path d="M18 3a3 3 0 0 0-3 3v12a3 3 0 0 0 3 3 3 3 0 0 0 3-3 3 3 0 0 0-3-3H6a3 3 0 0 0-3 3 3 3 0 0 0 3 3 3 3 0 0 0 3 3h12a3 3 0 0 0 3-3 3 3 0 0 0-3-3z"/></svg>';

  span.innerHTML = `${iconSvg}${entity.label}`;

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

export const parseContentToPayload = (container: HTMLDivElement): string => {
  let text = "";
  container.childNodes.forEach((node) => {
    if (node.nodeType === Node.TEXT_NODE) {
      text += node.textContent;
    } else if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as HTMLElement;
      if (el.hasAttribute("data-mention-id")) {
        const id = el.getAttribute("data-mention-id");
        const type = el.getAttribute("data-mention-type");
        const label = el.getAttribute("data-mention-label");
        // 生成格式: @[Label](Type:ID)
        text += ` @[${label}](${type}:${id}) `;
      } else {
        text += el.innerText;
      }
    }
  });
  return text
    .replace(/\u00A0/g, " ")
    .replace(/\s+/g, " ")
    .trim();
};

// 模拟命令响应（不使用 Gemini）
export const simulateCommandResponse = async (
  userPrompt: string,
): Promise<{ text: string; command?: any }> => {
  // 模拟处理延迟
  await new Promise((r) => setTimeout(r, 1000));

  // 检查是否包含提及
  const mentionRegex = /@\[([^\]]+)\]\(([^:]+):([^)]+)\)/g;
  const mentions = [];
  let match;
  while ((match = mentionRegex.exec(userPrompt)) !== null) {
    mentions.push({
      label: match[1],
      type: match[2],
      id: match[3],
    });
  }

  // 检查是否包含命令
  const commandRegex = /\/(filter|search|organize)/g;
  const command = commandRegex.exec(userPrompt);
  const commandType = command ? command[1] : null;

  // 生成模拟响应
  let responseText = "";
  let commandPayload = null;

  if (commandType === "filter" && mentions.length > 0) {
    // 模拟过滤命令
    const count = Math.floor(Math.random() * 50) + 10;
    responseText = `Found ${count} photos matching your criteria.`;
    commandPayload = {
      type: "filter_view",
      params: {
        [mentions[0].type + "_id"]: mentions[0].id,
      },
      count,
    };
  } else if (commandType === "search") {
    // 模拟搜索命令
    const count = Math.floor(Math.random() * 100) + 20;
    responseText = `Found ${count} photos matching your search query.`;
    commandPayload = {
      type: "search",
      count,
    };
  } else if (commandType === "organize") {
    // 模拟组织命令
    responseText = "I've organized your photos as requested.";
    commandPayload = {
      type: "organize",
    };
  } else if (mentions.length > 0) {
    // 有提及但没有命令
    responseText = `I see you're interested in ${mentions.map(m => m.label).join(", ")}. What would you like me to do with these?`;
  } else {
    // 普通查询
    responseText = "I understand your request. I can help you organize and find photos. Try using @ to mention albums, tags, cameras, lenses, or locations, or use commands like /filter, /search, or /organize.";
  }

  return { text: responseText, command: commandPayload };
};

# RichMention Component

RichMention 是一个 React 组件，实现了类似现代聊天应用中的 @提及 和 /命令功能，专为 Lumilio 照片管理应用设计。

## 功能概述

- **@提及功能**：用户可以输入 `@` 来提及不同类型的实体（相册、标签、相机、镜头、位置）
- **/命令功能**：用户可以输入 `/` 来触发命令菜单（过滤、搜索、组织等）
- **胶囊式展示**：选中的实体以带有图标的胶囊形式展示
- **命令响应**：支持模拟命令响应，无需外部 AI 服务

## 组件结构

```
RichMention/
├── components/          # React 组件
│   ├── RichMentionInput.tsx
│   └── MentionMenu.tsx
├── data/               # 模拟数据和图标
│   └── mockData.tsx
├── hooks/              # 自定义 React Hook
│   └── useRichMention.tsx
├── types/              # TypeScript 类型定义
│   └── index.ts
├── utils/              # 工具函数
│   └── index.ts
└── docs/               # 文档
    └── README.md
```

## 使用方法

### 基本使用

```tsx
import { RichMentionInput } from "./components/RichMention";

function MyChatComponent() {
  const handleSendMessage = (payload, response) => {
    console.log("Payload:", payload);
    console.log("Response:", response);
  };
  
  return (
    <RichMentionInput
      onSendMessage={handleSendMessage}
      isGenerating={false}
    />
  );
}
```

### 与 LumenChat 集成

LumenChat 组件已经集成了 RichMention 功能，用户可以在"Standard Input"和"Rich Input"之间切换。

```tsx
import { LumenChat } from "./components/LumenChat";

function App() {
  return <LumenChat />;
}
```

## API 参考

### RichMentionInput Props

| 属性 | 类型 | 描述 |
|------|------|------|
| onSendMessage | (payload: string, response?: {text: string, command?: any}) => void | 发送消息的回调函数 |
| isGenerating | boolean | 是否正在生成响应 |
| disabled | boolean | 是否禁用输入 |
| className | string | 自定义 CSS 类名 |

### useRichMention Hook

返回的值：

| 属性 | 类型 | 描述 |
|------|------|------|
| editorRef | RefObject<HTMLDivElement> | 编辑器 DOM 引用 |
| phase | TriggerPhase | 当前阶段（IDLE, SELECT_TYPE, SELECT_ENTITY, COMMAND） |
| menuPos | {top: number, left: number} \| null | 菜单位置 |
| selectedIndex | number | 当前选中的选项索引 |
| options | any[] | 当前可用的选项 |
| payload | string | 解析后的内容 |
| handleInput | () => void | 处理输入事件 |
| handleKeyDown | (e: React.KeyboardEvent) => void | 处理键盘事件 |
| handleSubmit | () => Promise<{text: string, command?: any}> | 提交消息 |
| clearContent | () => void | 清空内容 |

## 交互流程

1. 用户在输入框中输入 `@` 或 `/`
2. 系统检测到触发符，显示相应菜单
3. 用户选择类型/实体/命令：
   - 输入 `@` 后先选择类型（相册、标签等），再选择具体实体
   - 输入 `/` 后直接选择命令
4. 系统创建胶囊并插入到输入框
5. 用户提交查询后，内容被解析并发送给处理函数
6. 系统返回响应和可能的命令执行结果

## 自定义

### 添加新的实体类型

1. 在 `types/index.ts` 中扩展 `MentionType` 类型
2. 在 `data/mockData.tsx` 中添加新的图标和数据
3. 在 `utils/index.ts` 中更新解析逻辑

### 添加新的命令

1. 在 `data/mockData.tsx` 中的 `COMMANDS` 数组中添加新命令
2. 在 `utils/index.ts` 的 `simulateCommandResponse` 函数中添加处理逻辑

### 自定义样式

RichMention 使用 Tailwind CSS 类，可以通过以下方式自定义：

- 胶囊样式：修改 `useRichMention` 中的 `span.className`
- 菜单样式：修改 `MentionMenu` 组件中的类名

## 示例

### 基本示例

```tsx
import { useState } from "react";
import { RichMentionInput } from "./components/RichMention";

function Example() {
  const [messages, setMessages] = useState([]);
  
  const handleSendMessage = (payload, response) => {
    const userMessage = { role: "user", content: payload };
    setMessages(prev => [...prev, userMessage]);
    
    if (response) {
      const assistantMessage = { 
        role: "assistant", 
        content: response.text,
        command: response.command
      };
      setMessages(prev => [...prev, assistantMessage]);
    }
  };
  
  return (
    <div>
      <div className="message-list">
        {messages.map((msg, idx) => (
          <div key={idx} className={msg.role}>
            {msg.content}
          </div>
        ))}
      </div>
      <RichMentionInput
        onSendMessage={handleSendMessage}
        isGenerating={false}
      />
    </div>
  );
}
```

### 完整示例

查看 `pages/RichMentionDemo.tsx` 获取完整的示例实现。

## 注意事项

1. RichMention 使用 `contentEditable` div 而不是 textarea，以支持富文本内容
2. 胶章式提及是不可编辑的，光标会自动移动到胶囊后面
3. 内容会被解析为特定格式：`@[Label](Type:ID)`
4. 命令响应是模拟的，不依赖外部 AI 服务

## 故障排除

1. 如果菜单不显示，请检查是否正确导入了所有组件
2. 如果样式不正确，请确保 Tailwind CSS 已正确配置
3. 如果键盘导航不工作，请检查事件处理函数是否正确绑定
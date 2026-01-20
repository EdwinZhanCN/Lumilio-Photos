# RichInput

一个支持提及（@）和命令（/）的富文本输入框组件，使用 ContentEditable 实现灵活的文本编辑体验。

## 架构概览

RichInput 采用 **Context + Reducer** 模式进行状态管理：

- **RichInputProvider**: 状态管理的容器，提供全局状态
- **RichInputContext**: React Context，用于状态共享
- **RichInputReducer**: 状态逻辑处理器
- **useRichInput Hook**: 访问状态和 dispatch 的便捷方式

## 状态管理

### 状态结构

```typescript
interface RichInputState {
  phase: "IDLE" | "SELECT_TYPE" | "SELECT_ENTITY" | "COMMAND";  // 当前阶段
  activeMentionType: MentionType | null;  // 当前选中的提及类型
  options: MentionEntity[];  // 菜单选项列表
  selectedIndex: number;  // 当前选中的选项索引
  menuPos: { top: number; left: number } | null;  // 菜单位置
  payload: string;  // 解析后的内容（格式：@[Label](Type:ID)）
}
```

### Provider 使用

```tsx
import { RichInputProvider, RichInput } from "./RichInput";

function App() {
  return (
    <RichInputProvider>
      <YourComponent />
    </RichInputProvider>
  );
}
```

### Hook 使用

```tsx
import { useRichInput } from "./RichInput";

function YourComponent() {
  const { state, dispatch } = useRichInput();
  
  // 访问状态
  console.log(state.phase);
  console.log(state.payload);
  
  // 分发 action
  dispatch({ type: "SET_PHASE", payload: "SELECT_TYPE" });
}
```

## RichInput 组件

### 基础用法

```tsx
<RichInput
  placeholder="Type @ to mention albums..."
  mentionTypes={mentionTypes}
  getEntitiesByType={(type) => getEntities(type)}
  commands={commands}
  onSubmit={(payload) => console.log(payload)}
/>
```

### Props

| Prop | 类型 | 必填 | 说明 |
|------|------|------|------|
| `placeholder` | `string` | ❌ | 输入框占位符文本 |
| `mentionTypes` | `MentionTypeOption[]` | ❌ | 可用的提及类型 |
| `getEntitiesByType` | `(type) => MentionEntity[]` | ❌ | 根据类型获取实体列表 |
| `commands` | `MentionEntity[]` | ❌ | 可用的命令列表 |
| `onSubmit` | `(payload) => void` | ❌ | 提交回调 |
| `isSubmitting` | `boolean` | ❌ | 是否禁用提交 |
| `className` | `string` | ❌ | 自定义样式类名 |

### 使用示例

```tsx
import { RichInput } from "./RichInput";
import { MentionType } from "./types";

// 定义提及类型
const mentionTypes: MentionTypeOption[] = [
  { type: "album", label: "Album" },
  { type: "tag", label: "Tag" },
  { type: "location", label: "Location" },
];

// 定义命令
const commands: MentionEntity[] = [
  { id: "help", label: "Help", type: "command", desc: "查看帮助" },
  { id: "clear", label: "Clear", type: "command", desc: "清空内容" },
];

// 获取实体列表的函数
const getEntitiesByType = (type: MentionType) => {
  const entities: Record<MentionType, MentionEntity[]> = {
    album: [
      { id: "1", label: "Summer Trip", type: "album", icon: <IconAlbum /> },
      { id: "2", label: "Winter Holiday", type: "album", icon: <IconAlbum /> },
    ],
    tag: [
      { id: "1", label: "Beach", type: "tag", icon: <IconTag /> },
      { id: "2", label: "Mountain", type: "tag", icon: <IconTag /> },
    ],
    location: [
      { id: "1", label: "Paris", type: "location", icon: <IconLocation /> },
    ],
  };
  return entities[type] || [];
};

function ChatInput() {
  const handleSubmit = (payload: string) => {
    console.log("Submitted payload:", payload);
    // 处理提交，例如：@[Summer Trip](album:1) @[Beach](tag:2)
  };
  
  return (
    <RichInput
      placeholder="Ask Lumilio... (Type @ or /)"
      mentionTypes={mentionTypes}
      getEntitiesByType={getEntitiesByType}
      commands={commands}
      onSubmit={handleSubmit}
    />
  );
}
```

## Payload 格式

提交时，`onSubmit` 回调会收到格式化的 payload：

```
Hello @[Summer Trip](album:1) from @[Beach](tag:2)!
```

这种格式可以被后端轻松解析，提取出所有的提及实体。

## 工具函数

### `createPillElement(entity)`

创建一个胶囊元素（Pill），支持自定义图标：

```tsx
import { createPillElement } from "./utils";

const pill = createPillElement({
  id: "123",
  label: "Summer Trip",
  type: "album",
  icon: <CustomIcon />  // 可选，不传则使用默认 @ 图标
});
```

### `parseContentToPayload(container)`

将 contentEditable 的内容解析为 payload 格式。

### `clearEditor(editorRef)`

清空编辑器内容。

## 注意事项

1. **必须包裹在 Provider 中**: 使用 `RichInput` 组件前，必须用 `RichInputProvider` 包裹你的应用
2. **ContentEditable 限制**: 由于使用原生 DOM API，样式和事件处理需要特别小心
3. **图标自定义**: 可以通过 `icon` 字段自定义每个实体的图标，不传则使用默认的 @ 图标
4. **Payload 解析**: payload 格式为 `@[Label](Type:ID)`，后端需要相应处理
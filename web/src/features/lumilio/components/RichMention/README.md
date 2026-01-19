完成的主要工作

1. **创建了完整的 RichMention 组件系统**：
   - RichMentionInput：支持 @提及 和 /命令的富文本输入组件
   - MentionMenu：显示选择类型的下拉菜单
   - useRichMention：处理 richmention 逻辑的自定义 Hook

2. **实现了类型定义和数据结构**：
   - 定义了 MentionType, TriggerPhase, MentionEntity 等类型
   - 创建了模拟数据，包括相册、标签、相机、镜头、位置等实体
   - 定义了命令数据（filter, search, organize）

3. **开发了工具函数**：
   - parseContentToPayload：将富文本内容解析为有效载荷
   - simulateCommandResponse：模拟命令响应，不使用 Gemini

4. **集成到 LumenChat 中**：
   - 创建了 LumenRichInput 组件，将 RichMentionInput 集成到 LumenChat 中
   - 创建了 GalleryView 组件，显示命令执行结果的画廊视图
   - 实现了双模式输入，用户可以在标准输入和富输入之间切换

5. **对齐了输入框设计**：
   - 修改了输入区域样式，使其与原始示例完全一致
   - 调整了菜单定位方式和样式
   - 使用 CSS 伪元素实现占位符

6. **创建了演示页面**：
   - 创建了 RichMentionDemo 页面，用于展示和测试 richmention 功能

7. **编写了文档**：
   - 创建了详细的 README 文档，解释如何使用和自定义 RichMention 功能

### 功能特点

1. **@提及功能**：
   - 用户可以输入 `@` 来提及不同类型的实体（相册、标签、相机、镜头、位置）
   - 采用两级选择：先选择类型，再选择具体实体
   - 选中的实体以"胶囊"(pill)形式展示，包含图标和标签

2. **/命令功能**：
   - 用户可以输入 `/` 来触发命令菜单
   - 预定义了过滤、搜索、组织等命令
   - 命令执行后在画廊视图中显示结果

3. **模拟响应**：
   - 不使用 Gemini，而是通过 `simulateCommandResponse` 函数模拟响应
   - 根据用户输入的内容和提及的实体生成相应的响应
   - 支持侧信道数据（命令执行结果）的展示

4. **键盘导航**：
   - 使用箭头键导航菜单
   - 使用 Enter 或 Tab 选择选项
   - 使用 Escape 取消选择

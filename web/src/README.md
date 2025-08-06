# 项目结构

## `src/`
```txt
src/
├── features/            # 核心业务功能模块
├── components/          # 通用的、纯UI的组件 (你的“设计系统”)
├── lib/                 # 库/通用逻辑 (如API客户端、通用工具函数)
├── hooks/               # 通用的、与业务无关的自定义Hook
├── services/            # API服务层定义
├── styles/              # 全局样式
├── types/               # 全局共享的类型定义
├── routes/              # 路由配置
├── workers/             # workers/WorkerClient
├── wasm/                # WASM脚本和模组
├── App.tsx              # 主App入口
├── main.tsx             # 根入口
└── vite-env.d.ts        # Vite配置 (For Vercel)
```

### `features`

Example

```
src/
└── features/
    └── settings/
        ├── index.ts              // 出口: export { SettingsProvider, useSettings } from '...'
        ├── SettingsProvider.tsx  // Provider 组件
        ├── hooks/
        │   └── useSettings.ts    // 自定义 Hook
        ├── reducers/
        │   ├── lumen.reducer.ts
        │   └── ui.reducer.ts
        ├── settings.reducer.ts   // 根 Reducer
        └── types.ts              // Settings 相关的类型
```

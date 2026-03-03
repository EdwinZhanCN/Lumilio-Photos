# Studio Plugins Workspace

这个目录用于维护 Studio 的 CDN 插件包（当前为官方签名插件模式）。

当前示例插件：
- `lumilio-border-plugin`

## 当前系统架构（已落地）

Studio 插件系统由 5 部分组成：

1. 插件市场与安装态（主线程 UI）
- 文件：`web/src/features/studio/routes/Studio.tsx`
- `marketplace` 面板会拉取 catalog，并允许 install/uninstall。
- `plugins` 面板用于选择已安装插件并执行。
- 安装状态只保存在本地 `localStorage`：
  - key: `lumilio.studio.installed_plugins.v1`
  - 文件：`web/src/features/studio/plugins/installStore.ts`

2. Registry 客户端与校验链
- 文件：`web/src/features/studio/plugins/registryClient.ts`
- 会读取：
  - `VITE_PLUGIN_REGISTRY_URL`
  - `VITE_PLUGIN_CDN_ORIGIN`
- 会执行：
  - manifest 结构校验（`manifestGuard.ts`）
  - URL allowlist 校验（基于 `VITE_PLUGIN_CDN_ORIGIN`）
  - ECDSA P-256 SHA-256 签名校验（`signature.ts`）
  - revocation 校验（`/v1/revocations`）
- 公钥 ring 内置在前端：
  - 文件：`web/src/features/studio/plugins/keyring.ts`

3. UI Entry 动态加载（主线程）
- 文件：`web/src/features/studio/plugins/uiLoader.ts`
- 宿主动态 import `manifest.entries.ui`，并验证导出契约：
  - `meta`
  - `defaultParams`
  - `Panel`
  - `normalizeParams?`

4. Runner Entry 计算通道（Plugin Worker）
- Worker 文件：`web/src/workers/plugin.worker.ts`
- Client 文件：`web/src/workers/workerClient.ts`
- 消息协议：
  - `LOAD_RUNNER`
  - `RUN_PLUGIN`
  - `ABORT`
  - `PLUGIN_PROGRESS`
  - `PLUGIN_COMPLETE`
  - `ERROR`
- Worker 内部缓存 runner（`pluginId@version`），避免重复加载。
- 当前实现是单活跃任务模型（一次一个 active request）。

5. React 单例共享（避免双 React）
- import map：`web/index.html`
- shim：
  - `web/public/plugin-shims/react.js`
  - `web/public/plugin-shims/react-jsx-runtime.js`
- 宿主在 `web/src/main.tsx` 注入全局 React 单例供插件复用。

## 端到端执行流程

1. Marketplace 面板拉取 catalog。
2. 用户 install 某个插件版本（写入 localStorage）。
3. 进入 Plugins 工作区后，拉取并校验 manifest（结构/签名/白名单/revocation）。
4. 主线程加载 UI entry，渲染参数面板。
5. 点击 Apply 后，主线程调用 `runStudioPlugin`。
6. `plugin.worker` 加载 runner 并执行 wasm 计算。
7. 返回输出 Blob，Studio 展示处理结果。

## 插件契约（Runtime）

以 `web/src/features/studio/plugins/types.ts` 为准：

- `RuntimeManifestV1`
- `StudioPluginUiModule`
- `StudioPluginRunnerModule`
- `PluginRunResult`

manifest 关键字段：
- `entries.ui`
- `entries.runner`
- `mount.panel`
- `compatibility.studioApi`
- `signature`

## Border 插件现状

目录：`plugins/lumilio-border-plugin`

- `src/ui.tsx`：参数面板
- `src/runner.ts`：worker 内执行逻辑
- `src/vendor/*`：wasm-bindgen 产物（runner 源码直接引用）
- `dist/*`：CDN 发布产物
- `manifest.json`：发布 manifest

当前 border 行为：
- 输入支持：JPEG / PNG / WebP
- 输出统一：PNG（保留透明通道）
- decode 失败有 fallback（先浏览器侧转 PNG，再重试 wasm）
- 重模式（FROSTED/VIGNETTE）对超大图会预缩放，降低延迟和内存风险

## 本地开发流程（Border）

1. 重建 Rust wasm：

```bash
cd /Users/zhanzihao/Lumilio-Photos/wasm/border-wasm
wasm-pack build --target web --release --out-dir pkg --mode no-install --no-opt
```

2. 同步 wasm 到插件：

```bash
cp /Users/zhanzihao/Lumilio-Photos/wasm/border-wasm/pkg/border_wasm.js /Users/zhanzihao/Lumilio-Photos/plugins/lumilio-border-plugin/src/vendor/border_wasm.js
cp /Users/zhanzihao/Lumilio-Photos/wasm/border-wasm/pkg/border_wasm_bg.wasm /Users/zhanzihao/Lumilio-Photos/plugins/lumilio-border-plugin/src/vendor/border_wasm_bg.wasm
```

3. 重新打包 dist（用于 CDN 发布物）：

```bash
cd /Users/zhanzihao/Lumilio-Photos
node_modules/.bin/esbuild plugins/lumilio-border-plugin/src/ui.tsx --bundle --format=esm --platform=browser --target=es2022 --external:react --external:react/jsx-runtime --outfile=plugins/lumilio-border-plugin/dist/ui.mjs
node_modules/.bin/esbuild plugins/lumilio-border-plugin/src/runner.ts --bundle --format=esm --platform=browser --target=es2022 --outfile=plugins/lumilio-border-plugin/dist/runner.mjs
```

说明：
- 根目录 `.gitignore` 包含 `dist/`，所以 `plugins/**/dist` 默认不会入 git。

## Cloudflare 发布链路（当前代码位置）

- Registry Worker：`infra/cloudflare/registry-worker`
- 发布脚本：`infra/cloudflare/scripts/publish-plugin.mjs`
- 典型 API：
  - `GET /v1/catalog?panel=plugins`
  - `GET /v1/plugins/:pluginId/manifest`
  - `GET /v1/plugins/:pluginId/manifest/:version`
  - `GET /v1/revocations`

发布示例：

```bash
cd /Users/zhanzihao/Lumilio-Photos
node infra/cloudflare/scripts/publish-plugin.mjs \
  --plugin-id com.lumilio.border \
  --version 0.1.0 \
  --source plugins/lumilio-border-plugin/dist \
  --manifest plugins/lumilio-border-plugin/manifest.json \
  --bucket lumilio-plugin-artifacts \
  --db lumilio_plugin_registry
```

## 当前边界

- 当前是“官方签名插件”模型，非第三方开放市场。
- `permissions` 字段目前仅做 manifest 校验，不做 runtime 权限隔离。
- 安装态仅本地，不做账号同步。

# Makefile → Taskfile 迁移（已完成）

## 目标

用 [Task](https://taskfile.dev/)（Taskfile v3）全面替换根 Makefile 体系，统一并简化自动构建流程：

- 根目录只保留一个编排层（`taskfile.yml`），业务逻辑下沉到 `server/`、`web/`、`desktop/`、`site/` 各自的 Taskfile（命名空间化）。
- CI/CD（`.github/workflows/*.yml`）全面改为调用 `task`。
- 删除 Makefile 及其胶水层；`package.json` scripts 作为前端叶子层保留。
- 保留两个安全关键脚本：`scripts/dev-state.sh`（.local 开发根目录的所有权/标记/symlink 防护）和 `scripts/check-architecture.sh`（SQL 与浏览/搜索契约静态回归）。

## 最终契约

### 文件结构

```
├── taskfile.yml                  # 根编排层：setup/dev/test/dto/compose-test/architecture-check/dev-config/dev-clean/dev-reset/dev-purge
├── server/taskfile.yml           # install/test/dev/config-init(internal)/dto/config-examples/sqlc
├── web/taskfile.yml              # install/dev/build/lint/type-check/check:boundaries/test + e2e:* + test:{browser,auth-hardening,video-semantic,backup-recovery,coverage,watch} + dto:types/assets:sync/i18n:extract/demo:seed
├── desktop/Taskfile.yml          # Wails3 生成（build/package/run/dev，大写原名不动）+ 追加 test/resources-manifest
└── site/taskfile.yml             # install/dev/build/preview/dto:redoc + media:{manifest,sync,verify}
```

所有新建 Taskfile 使用小写 `taskfile.yml`（用户约定）。`desktop/Taskfile.yml` 是 Wails3 生成物，保持大写原名，只追加不覆盖。

### 任务映射（make → task，最终契约）

| make（已删除） | task |
| --- | --- |
| `make setup` | `task setup`（串行：server:install → dev:config → web:install → site:install → tools:install → hooks:install） |
| `make dev` | `task dev`（dev:config → dev:services 并行 server:dev + web:dev） |
| `make server-dev` / `make web-dev` | `task server:dev` / `task web:dev` |
| `make test` | `task test`（architecture:check + 并行 server:test + web:test） |
| `make server-test` | `task server:test` |
| `make web-test` | `task web:test` |
| `make web-browser-test` | `task web:test:browser` |
| `make web-auth-hardening-test` | `task web:test:auth-hardening` |
| `make web-video-semantic-test` | `task web:test:video-semantic` |
| `make web-backup-recovery-test` | `task web:test:backup-recovery` |
| `make architecture-check` | `task architecture:check`（独立门禁，不再绑定 server:test） |
| `make compose-test` | `task compose:test` |
| `make dto` | `task dto`（串行 server:openapi → web:openapi-types → site:openapi-docs） |
| `make config-examples` | `task config:examples`（聚合 server:config-examples） |
| `make dev-config` | `task dev:config` |
| `make dev-clean` / `dev-reset` / `dev-purge` | `task dev:clean` / `dev:reset` / `dev:purge`（purge 用 `prompt:` 交互确认，脚本 `CONFIRM_DEV_PURGE` 守卫保留） |
| `make desktop-dev` / `desktop-build` | `task desktop:dev` / `desktop:build`（既有 Wails3 任务） |
| `make desktop-test` | `task desktop:test`（本地，-race） |
| `make desktop-resources-manifest` | `task desktop:resources-manifest` |

新增任务：`tools:install`（wasm-pack/swag，`status:` 跳过已装）、`hooks:install`（`if:` git 仓库）、`server:test:ci`（windows，go clean -cache）、`desktop:test:ci`（CI 版，go clean -cache）、`desktop:compile`、`desktop:frontend:install:ci`/`frontend:build`（控制面板）、`web:playwright:install`/`install:deps`、`web:install:ci`/`site:install:ci`（frozen lockfile）、`ci:architecture`/`ci:server`/`ci:web`/`ci:site`/`ci:desktop:panel`/`ci:desktop:native`（CI 契约）。

### 职责边界核查（最终状态）

- Actions 保留：runner/OS、APT/Homebrew/MSYS2（含 exiftool stub 与 go version/pkgconf 工具链诊断）、setup-go/setup-vp、GitHub Packages 凭据、Playwright/Docker 缓存、Buildx、artifacts（含 E2E 失败日志收集）、path filters、ci-required 汇总。
- Taskfile 负责：目录切换（include dir）、Go build tags、平台测试参数（`CGO_ENABLED=1`、`GOFLAGS=-buildvcs=false` 已从 ci.yml 移入 server/desktop 任务 env）、Server/Web/Desktop/Site 命令、测试顺序、并行 dev 服务、DTO 生成、Compose 校验、E2E 命令入口（含 playwright 安装命令）。

### CI 契约

- 需要 task 的 job（sqlite-architecture、server-linux、web、desktop-macos）统一用：
  ```yaml
  - uses: go-task/setup-task@v2
    with:
      version: 3.52.0
  ```
- `desktop-windows` job 保持 msys2 内联（go test/go build），不迁移：`check-architecture.sh` 依赖 `rg`，msys2 工具链未提供，且该 job 原无 make 调用。
- `changes` job 的 paths-filter 中 `'Makefile'` → `'taskfile.yml'`（根 taskfile 变更触发全量相关 gate）。
- e2e `vp run e2e:up/down`、`vp install --frozen-lockfile` 等 CI 内联步骤保留（CI 自有节奏）。

### 配套改动

- `scripts/dev-state.sh`：仓库断言改为 `taskfile.yml`；`dev-purge` 错误信息改为 `task dev-purge`。
- `scripts/check-architecture.sh`：未改动（由根任务调用）。
- 根 `.gitignore` 增加 `/.task/`（Task 指纹目录；desktop/.gitignore 已有）。
- 文档全部替换 make 引用：AGENTS.md、CLAUDE.md、CONTRIBUTING.md（含前置条件 Make → Task）、BACKEND.md、FRONTEND.md、docts.md、architecture.md、site/README.md、desktop-legacy/resources/README.md、desktop-legacy/scripts/build-macos.sh 错误信息、desktop-wails3-migration.md 中的两处提法。

### 验证边界

- `task --list` / `task --list-all`：全量任务带 desc，命名空间与别名正确（s:/w:/d:）。
- `task dev:config`：dev-state.sh init + server:config-init 生成 `.local/dev/config/server.toml`。
- `task ci:architecture`（architecture:check + compose:test）：通过。
- `task server:test`、`task desktop:test:ci`、`task desktop:compile`、`task tools:install`（status 跳过）：通过。
- `task test`：architecture:check + server:test + web:test 全量通过（293 tests）。
- `task dev` 冒烟：并行拉起 server + web（web 200；server 日志正常，健康检查时机需晚于首次启动迁移）。
- 本地 sharp postinstall 失败（macOS 27 预发布 + Node 24 下 sharp 0.34.5 prebuilt 兼容性）为环境问题，与迁移无关；CI ubuntu 不受影响。
- CI 的 `task ci:*` 调用与 setup-task@v2 安装需推送后由 workflow 验证。

## 有用决策

1. **include 必须显式 `dir:`**：Task 默认让被 include 的任务在父 Taskfile 目录运行，否则 `server:install` 会在根目录执行。四个 include 均声明 `dir:`。
2. **`cmds` 不支持 `sh:` 键**（那是 `preconditions` 语法），用 `cmd:` 结构化形式。
3. **嵌套任务名与顶层键冲突**：`dev:clean` 的 YAML 嵌套写法会覆盖顶层 `dev` 任务，改为扁平 `dev-clean`/`dev-reset`/`dev-purge`。
4. **`web:install`/`site:install` 需要 `CI=1 VITE_GIT_HOOKS=0` env**：与 `make setup` 原命令等价（sharp 强制预编译、跳过 vite-plus git hooks）。
5. **CGO env 双写**：`CGO_LDFLAGS_ALLOW`/`CGO_CFLAGS_ALLOW` 在根、server、desktop 各自声明（根 env 是否透传 include 未验证，双写成本低、语义显式）。
6. **`dev-purge` 用 `prompt:` + 脚本守卫双保险**；CI/非 TTY 环境用 `task --yes dev-purge`。
7. **Task 版本**：本地 brew（3.52.0）与 CI setup-task@v2（3.52.0）对齐；Taskfile 声明 `version: '3'` 兜底最低版本。
